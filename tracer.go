package elasticapm

import (
	"fmt"
	"log"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/elastic/apm-agent-go/model"
	"github.com/elastic/apm-agent-go/stacktrace"
	"github.com/elastic/apm-agent-go/transport"
)

const (
	defaultPreContext      = 3
	defaultPostContext     = 3
	transactionsChannelCap = 1000
	errorsChannelCap       = 1000
)

var (
	// DefaultTracer is the default global Tracer, set at package
	// initialization time, configured via environment variables.
	//
	// This will always be initialized to a non-nil value. If any
	// of the environment variables are invalid, the corresponding
	// errors will be logged to stderr and the default values will
	// be used instead.
	DefaultTracer *Tracer
)

func init() {
	var opts options
	opts.init(true)
	DefaultTracer = newTracer(opts)
}

type options struct {
	requestDuration       time.Duration
	metricsInterval       time.Duration
	maxSpans              int
	sampler               Sampler
	sanitizedFieldNames   *regexp.Regexp
	captureBody           CaptureBodyMode
	spanFramesMinDuration time.Duration
	serviceName           string
	serviceVersion        string
	serviceEnvironment    string
	active                bool
	distributedTracing    bool
}

func (opts *options) init(continueOnError bool) error {
	var errs []error
	failed := func(err error) bool {
		if err == nil {
			return false
		}
		errs = append(errs, err)
		return true
	}

	requestDuration, err := initialRequestDuration()
	if failed(err) {
		requestDuration = defaultAPIRequestTime
	}

	metricsInterval, err := initialMetricsInterval()
	if err != nil {
		metricsInterval = defaultMetricsInterval
		errs = append(errs, err)
	}

	maxSpans, err := initialMaxSpans()
	if failed(err) {
		maxSpans = defaultMaxSpans
	}

	sampler, err := initialSampler()
	if failed(err) {
		sampler = nil
	}

	sanitizedFieldNames, err := initialSanitizedFieldNamesRegexp()
	if failed(err) {
		sanitizedFieldNames = defaultSanitizedFieldNames
	}

	captureBody, err := initialCaptureBody()
	if failed(err) {
		captureBody = CaptureBodyOff
	}

	spanFramesMinDuration, err := initialSpanFramesMinDuration()
	if failed(err) {
		spanFramesMinDuration = defaultSpanFramesMinDuration
	}

	active, err := initialActive()
	if failed(err) {
		active = true
	}

	distributedTracing, err := initialDistributedTracing()
	if failed(err) {
		distributedTracing = false
	}

	if len(errs) != 0 && !continueOnError {
		return errs[0]
	}
	for _, err := range errs {
		log.Printf("[elasticapm]: %s", err)
	}

	opts.requestDuration = requestDuration
	opts.metricsInterval = metricsInterval
	opts.maxSpans = maxSpans
	opts.sampler = sampler
	opts.sanitizedFieldNames = sanitizedFieldNames
	opts.captureBody = captureBody
	opts.spanFramesMinDuration = spanFramesMinDuration
	opts.serviceName, opts.serviceVersion, opts.serviceEnvironment = initialService()
	opts.active = active
	opts.distributedTracing = distributedTracing
	return nil
}

// Tracer manages the sampling and sending of transactions to
// Elastic APM.
//
// Transactions are buffered until they are flushed (forcibly
// with a Flush call, or when the flush timer expires), or when
// the maximum transaction queue size is reached. Failure to
// send will be periodically retried. Once the queue limit has
// been reached, new transactions will replace older ones in
// the queue.
//
// Errors are sent as soon as possible, but will buffered and
// later sent in bulk if the tracer is busy, or otherwise cannot
// send to the server, e.g. due to network failure. There is
// a limit to the number of errors that will be buffered, and
// once that limit has been reached, new errors will be dropped
// until the queue is drained.
//
// The exported fields be altered or replaced any time up until
// any Tracer methods have been invoked.
type Tracer struct {
	Transport transport.Transport
	Service   struct {
		Name        string
		Version     string
		Environment string
	}

	process *model.Process
	system  *model.System

	active             bool
	distributedTracing bool
	closing            chan struct{}
	closed             chan struct{}
	forceFlush         chan chan<- struct{}
	forceSendMetrics   chan chan<- struct{}
	configCommands     chan tracerConfigCommand
	transactions       chan *Transaction
	errors             chan *Error

	statsMu sync.Mutex
	stats   TracerStats

	maxSpansMu sync.RWMutex
	maxSpans   int

	spanFramesMinDurationMu sync.RWMutex
	spanFramesMinDuration   time.Duration

	samplerMu sync.RWMutex
	sampler   Sampler

	captureBodyMu sync.RWMutex
	captureBody   CaptureBodyMode

	errorPool       sync.Pool
	spanPool        sync.Pool
	transactionPool sync.Pool
}

// NewTracer returns a new Tracer, using the default transport,
// initializing a Service with the specified name and version,
// or taking the service name and version from the environment
// if unspecified.
//
// If serviceName is empty, then the service name will be defined
// using the ELASTIC_APM_SERVER_NAME environment variable.
func NewTracer(serviceName, serviceVersion string) (*Tracer, error) {
	var opts options
	if err := opts.init(false); err != nil {
		return nil, err
	}
	if serviceName != "" {
		if err := validateServiceName(serviceName); err != nil {
			return nil, err
		}
		opts.serviceName = serviceName
		opts.serviceVersion = serviceVersion
	}
	return newTracer(opts), nil
}

func newTracer(opts options) *Tracer {
	t := &Tracer{
		Transport:             transport.Default,
		process:               &currentProcess,
		system:                &localSystem,
		closing:               make(chan struct{}),
		closed:                make(chan struct{}),
		forceFlush:            make(chan chan<- struct{}),
		forceSendMetrics:      make(chan chan<- struct{}),
		configCommands:        make(chan tracerConfigCommand),
		transactions:          make(chan *Transaction, transactionsChannelCap),
		errors:                make(chan *Error, errorsChannelCap),
		maxSpans:              opts.maxSpans,
		sampler:               opts.sampler,
		captureBody:           opts.captureBody,
		spanFramesMinDuration: opts.spanFramesMinDuration,
		active:                opts.active,
		distributedTracing:    opts.distributedTracing,
	}
	t.Service.Name = opts.serviceName
	t.Service.Version = opts.serviceVersion
	t.Service.Environment = opts.serviceEnvironment

	if !t.active {
		close(t.closed)
		return t
	}

	go t.loop()
	t.configCommands <- func(cfg *tracerConfig) {
		cfg.metricsInterval = opts.metricsInterval
		cfg.requestDuration = opts.requestDuration
		cfg.sanitizedFieldNames = opts.sanitizedFieldNames
		cfg.preContext = defaultPreContext
		cfg.postContext = defaultPostContext
		cfg.metricsGatherers = []MetricsGatherer{newBuiltinMetricsGatherer(t)}
	}
	return t
}

// Close closes the Tracer, preventing transactions from being
// sent to the APM server.
func (t *Tracer) Close() {
	select {
	case <-t.closing:
	default:
		close(t.closing)
	}
	<-t.closed
}

// Flush waits for the Tracer to flush any transactions and errors it currently
// has queued to the APM server, the tracer is stopped, or the abort channel
// is signaled.
func (t *Tracer) Flush(abort <-chan struct{}) {
	flushed := make(chan struct{}, 1)
	select {
	case t.forceFlush <- flushed:
		select {
		case <-abort:
		case <-flushed:
		case <-t.closed:
		}
	case <-t.closed:
	}
}

// Active reports whether the tracer is active. If the tracer is inactive,
// no transactions or errors will be sent to the Elastic APM server.
func (t *Tracer) Active() bool {
	return t.active
}

// SetRequestDuration sets the maximum amount of time to keep a request open
// to the APM server for streaming data before closing the stream and starting
// a new request.
func (t *Tracer) SetRequestDuration(d time.Duration) {
	t.sendConfigCommand(func(cfg *tracerConfig) {
		cfg.requestDuration = d
	})
}

// SetMetricsInterval sets the metrics interval -- the amount of time in
// between metrics samples being gathered.
func (t *Tracer) SetMetricsInterval(d time.Duration) {
	t.sendConfigCommand(func(cfg *tracerConfig) {
		cfg.metricsInterval = d
	})
}

// SetContextSetter sets the stacktrace.ContextSetter to be used for
// setting stacktrace source context. If nil (which is the initial
// value), no context will be set.
func (t *Tracer) SetContextSetter(setter stacktrace.ContextSetter) {
	t.sendConfigCommand(func(cfg *tracerConfig) {
		cfg.contextSetter = setter
	})
}

// SetLogger sets the Logger to be used for logging the operation of
// the tracer.
func (t *Tracer) SetLogger(logger Logger) {
	t.sendConfigCommand(func(cfg *tracerConfig) {
		cfg.logger = logger
	})
}

// SetSanitizedFieldNames sets the patterns that will be used to match
// cookie and form field names for sanitization. Fields matching any
// of the the supplied patterns will have their values redacted. If
// SetSanitizedFieldNames is called with no arguments, then no fields
// will be redacted.
func (t *Tracer) SetSanitizedFieldNames(patterns ...string) error {
	var re *regexp.Regexp
	if len(patterns) != 0 {
		var err error
		re, err = regexp.Compile(fmt.Sprintf("(?i:%s)", strings.Join(patterns, "|")))
		if err != nil {
			return err
		}
	}
	t.sendConfigCommand(func(cfg *tracerConfig) {
		cfg.sanitizedFieldNames = re
	})
	return nil
}

// RegisterMetricsGatherer registers g for periodic (or forced) metrics
// gathering by t.
//
// RegisterMetricsGatherer returns a function which will deregister g.
// It may safely be called multiple times.
func (t *Tracer) RegisterMetricsGatherer(g MetricsGatherer) func() {
	// Wrap g in a pointer-to-struct, so we can safely compare.
	wrapped := &struct{ MetricsGatherer }{MetricsGatherer: g}
	t.sendConfigCommand(func(cfg *tracerConfig) {
		cfg.metricsGatherers = append(cfg.metricsGatherers, wrapped)
	})
	deregister := func(cfg *tracerConfig) {
		for i, g := range cfg.metricsGatherers {
			if g != wrapped {
				continue
			}
			cfg.metricsGatherers = append(cfg.metricsGatherers[:i], cfg.metricsGatherers[i+1:]...)
		}
	}
	var once sync.Once
	return func() {
		once.Do(func() {
			t.sendConfigCommand(deregister)
		})
	}
}

func (t *Tracer) sendConfigCommand(cmd tracerConfigCommand) {
	select {
	case t.configCommands <- cmd:
	case <-t.closing:
	case <-t.closed:
	}
}

// SetSampler sets the sampler the tracer. It is valid to pass nil,
// in which case all transactions will be sampled.
func (t *Tracer) SetSampler(s Sampler) {
	t.samplerMu.Lock()
	t.sampler = s
	t.samplerMu.Unlock()
}

// SetMaxSpans sets the maximum number of spans that will be added
// to a transaction before dropping. If set to a non-positive value,
// the number of spans is unlimited.
func (t *Tracer) SetMaxSpans(n int) {
	t.maxSpansMu.Lock()
	t.maxSpans = n
	t.maxSpansMu.Unlock()
}

// SetSpanFramesMinDuration sets the minimum duration for a span after which
// we will capture its stack frames.
func (t *Tracer) SetSpanFramesMinDuration(d time.Duration) {
	t.spanFramesMinDurationMu.Lock()
	t.spanFramesMinDuration = d
	t.spanFramesMinDurationMu.Unlock()
}

// SetCaptureBody sets the HTTP request body capture mode.
func (t *Tracer) SetCaptureBody(mode CaptureBodyMode) {
	t.captureBodyMu.Lock()
	t.captureBody = mode
	t.captureBodyMu.Unlock()
}

// SendMetrics forces the tracer to gather and send metrics immediately,
// blocking until the metrics have been sent or the abort channel is
// signalled.
func (t *Tracer) SendMetrics(abort <-chan struct{}) {
	sent := make(chan struct{}, 1)
	select {
	case t.forceSendMetrics <- sent:
		select {
		case <-abort:
		case <-sent:
		case <-t.closed:
		}
	case <-t.closed:
	}
}

// Stats returns the current TracerStats. This will return the most
// recent values even after the tracer has been closed.
func (t *Tracer) Stats() TracerStats {
	t.statsMu.Lock()
	stats := t.stats
	t.statsMu.Unlock()
	return stats
}

func (t *Tracer) loop() {
	defer close(t.closed)

	/*
		ctx, cancelContext := context.WithCancel(context.Background())
		defer cancelContext()
		go func() {
			select {
			case <-t.closing:
				cancelContext()
			}
		}()
	*/

	var cfg tracerConfig
	//var forceFlushed chan<- struct{}
	//var forceSentMetrics chan<- struct{}
	//var gatherMetricsC <-chan time.Time
	//var gatheringMetrics bool

	//forceSendMetrics := t.forceSendMetrics
	//gatheredMetrics := make(chan struct{}, 1)
	flushTimer := time.NewTimer(0)
	if !flushTimer.Stop() {
		<-flushTimer.C
	}
	metricsTimer := time.NewTimer(0)
	if !metricsTimer.Stop() {
		<-metricsTimer.C
	}
	/*
		startTimer := func(ch *<-chan time.Time, timer *time.Timer, interval time.Duration) {
			if *ch != nil {
				// Timer already started.
				return
			}
			if !timer.Stop() {
				select {
				case <-timer.C:
				default:
				}
			}
			if interval <= 0 {
				// Non-positive interval disables the timer.
				return
			}
			timer.Reset(interval)
			*ch = timer.C
		}
	*/
	startMetricsTimer := func() {
		//startTimer(&gatherMetricsC, metricsTimer, cfg.metricsInterval)
	}

	sender := newSender(t)
	cfgC := sender.cfgC
	//metricsC := sender.metrics

	/*
		sendError := func(e *Error) {
			sender.sendError(e)
			e.reset()
		}
		sendTransaction := func(tx *Transaction) {
			sender.sendTransaction(tx)
			tx.reset()
		}
	*/

	for {
		//var gatherMetrics bool
		//var sendMetrics bool

		/*
			forceFlush := t.forceFlush
			flushTimerC := sender.flushTimer.C
			if sender.sendingStream && !sender.streamOpen {
				// While we're flushing, we discard new errors
				// in favour of older ones, under the assumption
				// that newer errors are more likely to be due
				// to cascading failure.
				errorsC = nil
				forceFlush = nil
				flushTimerC = nil
			}
		*/

		select {
		case <-t.closing:
			return

		case cmd := <-t.configCommands:
			cmd(&cfg)
			cfgC = sender.cfgC
			startMetricsTimer()
			continue

		case cfgC <- cfg:
			cfgC = nil

		case stats := <-sender.statsC:
			t.statsMu.Lock()
			t.stats.accumulate(stats)
			t.statsMu.Unlock()

			/*
				case err := <-sender.sentStream:
					if err != nil && cfg.logger != nil {
						// TODO(axw) set/extend grace period deadline
						cfg.logger.Debugf("failed to send stream: %s", err)
					}
					sender.sendingStream = false
					if forceFlushed != nil {
						forceFlushed <- struct{}{}
						forceFlushed = nil
					}


				case e := <-errorsC:
					sendError(e)

				case tx := <-t.transactions:
					if sender.sendingStream && !sender.streamOpen {
						// While we're flushing we still accept
						// transactions, enqueuing them for when
						// we can start a new request.
						enqueueTransaction(tx)
						continue
					}
					sendTransaction(tx)

				case <-flushTimerC:
					closeStream = true
			*/

			/*
				case forceFlushed = <-forceFlush:
					// forceFlushed will be signaled when the current request
					// is successfully sent.
					closeStream = true
			*/

			/*
				case <-gatherMetricsC:
					gatherMetricsC = nil
					gatherMetrics = !gatheringMetrics
			*/

			/*
				case forceSentMetrics = <-forceSendMetrics:
					// forceSentMetrics will be signaled, and forceSendMetrics
					// set back to t.forceSendMetrics, when metrics have been
					// gathered and an attempt to send them has been made.
					forceSendMetrics = nil
					gatherMetricsC = nil
					gatherMetrics = !gatheringMetrics

			*/

			/*
				case <-gatheredMetrics:
					gatheringMetrics = false
					sendMetrics = true
			*/
		}

		/*
			if gatherMetrics {
				gatheringMetrics = true
				sender.gatherMetrics(ctx, gatheredMetrics)
			}

			if sendMetrics {
				sender.sendMetrics()
				// We don't retry sending metrics on failure;
				// inform the caller that an attempt was made
				// regardless of the outcome, and restart the
				// timer.
				if forceSentMetrics != nil {
					forceSentMetrics <- struct{}{}
					forceSentMetrics = nil
					forceSendMetrics = t.forceSendMetrics
				}
				startMetricsTimer()
			}
		*/
	}
}

/*
func (t *tracer) gatherMetrics(ctx context.Context, cfg tracerConfig, m *Metrics, gathered chan<- struct{}) {
	timestamp := model.Time(time.Now().UTC())
	var group sync.WaitGroup
	for _, g := range cfg.metricsGatherers {
		group.Add(1)
		go func(g MetricsGatherer) {
			defer group.Done()
			gatherMetrics(ctx, g, m, logger)
		}(g)
	}
	go func() {
		group.Wait()
		for _, m := range m.metrics {
			m.Timestamp = timestamp
		}
		gathered <- struct{}{}
	}()
}
*/

// tracerConfig holds the tracer's runtime configuration, which may be modified
// by sending a tracerConfigCommand to the tracer's configCommands channel.
type tracerConfig struct {
	requestDuration         time.Duration
	metricsInterval         time.Duration
	logger                  Logger
	metricsGatherers        []MetricsGatherer
	contextSetter           stacktrace.ContextSetter
	preContext, postContext int
	sanitizedFieldNames     *regexp.Regexp
}

type tracerConfigCommand func(*tracerConfig)
