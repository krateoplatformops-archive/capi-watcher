package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/krateoplatformops/capi-watcher/internal/handler"
	"github.com/krateoplatformops/capi-watcher/internal/recorder"
	"github.com/krateoplatformops/capi-watcher/internal/support"
	"github.com/krateoplatformops/capi-watcher/internal/watcher"
	"github.com/rs/zerolog"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

const (
	serviceName = "ObjectStatusWatcher"
)

var (
	Version string
	Build   string
)

func main() {
	// Flags
	flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ExitOnError)

	kubeconfig := flag.String(clientcmd.RecommendedConfigPathFlag, "", "absolute path to the kubeconfig file")
	debug := flag.Bool("debug",
		support.EnvBool("OBJECT_STATUS_WATCHER_DEBUG", false), "dump verbose output")
	resyncInterval := flag.Duration("resync-interval",
		support.EnvDuration("OBJECT_STATUS_WATCHER_RESYNC_INTERVAL", time.Minute*3), "resync interval")
	namespace := flag.String("namespace",
		support.EnvString("OBJECT_STATUS_WATCHER_NAMESPACE", ""), "namespace to list and watch")

	flag.Usage = func() {
		fmt.Fprintln(flag.CommandLine.Output(), "Flags:")
		flag.PrintDefaults()
	}

	flag.Parse()

	// Initialize the logger
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix

	// Default level for this log is info, unless debug flag is present
	zerolog.SetGlobalLevel(zerolog.InfoLevel)
	if *debug {
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
	}

	log := zerolog.New(os.Stdout).With().
		Str("service", serviceName).
		Timestamp().
		Logger()

	support.FixKubernetesServicePortEventually()

	// Kubernetes configuration
	var cfg *rest.Config
	var err error
	if len(*kubeconfig) > 0 {
		cfg, err = clientcmd.BuildConfigFromFlags("", *kubeconfig)
	} else {
		cfg, err = rest.InClusterConfig()
	}
	if err != nil {
		log.Fatal().Err(err).Msg("building kube config")
	}

	rec, err := recorder.New(cfg)
	if err != nil {
		log.Fatal().Err(err).Msg("building event recorder")
	}

	// Grab a dynamic interface that we can create informers from
	dc, err := dynamic.NewForConfig(cfg)
	if err != nil {
		log.Fatal().Err(err).Msg("could not generate dynamic client for config")
	}

	handler := handler.NewStatusChecker(handler.StatusCheckerOpts{
		Log:             log,
		ConditionName:   "Ready",
		ConditionStatus: "True",
		Recorder:        rec,
	})
	if err != nil {
		log.Fatal().Err(err).Msg("creating the object status handler")
	}

	gvr, _ := schema.ParseResourceArg("clusters.v1beta1.cluster.x-k8s.io")

	watcher := watcher.New(watcher.Opts{
		DynamicClient: dc,
		GVR: schema.GroupVersionResource{
			Group:    gvr.Group,
			Version:  gvr.Version,
			Resource: gvr.Resource,
		},
		Handler:   handler,
		Namespace: *namespace,
		Log:       log,
	})

	stop := sigHandler(log)

	var wg sync.WaitGroup
	wg.Add(1)

	go func() {
		defer wg.Done()

		log.Info().
			Str("version", Version).
			Bool("debug", *debug).
			Dur("resyncInterval", *resyncInterval).
			Str("namespace", *namespace).
			Msgf("Starting %s", serviceName)

		watcher.Run(stop)
	}()

	wg.Wait()
	log.Warn().Msgf("%s done", serviceName)
	os.Exit(1)
}

// setup a signal hander to gracefully exit
func sigHandler(log zerolog.Logger) <-chan struct{} {
	stop := make(chan struct{})
	go func() {
		c := make(chan os.Signal, 1)
		signal.Notify(c,
			syscall.SIGINT,  // Ctrl+C
			syscall.SIGTERM, // Termination Request
			syscall.SIGSEGV, // FullDerp
			syscall.SIGABRT, // Abnormal termination
			syscall.SIGILL,  // illegal instruction
			syscall.SIGFPE)  // floating point - this is why we can't have nice things
		sig := <-c
		log.Warn().Msgf("Signal (%v) detected, shutting down", sig)
		close(stop)
	}()
	return stop
}
