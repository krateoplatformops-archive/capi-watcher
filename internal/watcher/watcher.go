package watcher

import (
	"fmt"
	"time"

	"github.com/rs/zerolog"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/dynamic/dynamicinformer"
	"k8s.io/client-go/tools/cache"
)

type UnstructuredHandler interface {
	Handle(obj *unstructured.Unstructured)
}

type Dynamic struct {
	log      zerolog.Logger
	handler  UnstructuredHandler
	informer cache.SharedInformer
}

type Opts struct {
	DynamicClient  dynamic.Interface
	GVR            schema.GroupVersionResource
	Handler        UnstructuredHandler
	ResyncInterval time.Duration
	Log            zerolog.Logger
	Namespace      string
}

func New(opts Opts) *Dynamic {
	fac := dynamicinformer.NewFilteredDynamicSharedInformerFactory(
		opts.DynamicClient, opts.ResyncInterval, opts.Namespace, nil)

	sin := fac.ForResource(opts.GVR).Informer()

	return &Dynamic{
		informer: sin,
		handler:  opts.Handler,
		log:      opts.Log,
	}
}

func (er *Dynamic) Run(stopCh <-chan struct{}) {
	er.informer.AddEventHandler(
		cache.ResourceEventHandlerFuncs{
			AddFunc:    er.OnAdd,
			UpdateFunc: er.OnUpdate,
			DeleteFunc: er.OnDelete,
		},
	)

	defer utilruntime.HandleCrash()

	er.informer.Run(stopCh)

	// here is where we kick the caches into gear
	if !cache.WaitForCacheSync(stopCh, er.informer.HasSynced) {
		utilruntime.HandleError(fmt.Errorf("timed out waiting for caches to sync"))
		return
	}

	<-stopCh
}

func (er *Dynamic) OnAdd(obj interface{}) {
	el := obj.(*unstructured.Unstructured)

	er.log.Debug().
		Str("kind", el.GetKind()).
		Str("name", el.GetName()).
		Str("namespace", el.GetNamespace()).
		Msg("received add event")

	er.handler.Handle(el.DeepCopy())
}

func (er *Dynamic) OnUpdate(objOld interface{}, objNew interface{}) {
	el := objNew.(*unstructured.Unstructured)

	er.log.Debug().
		Str("kind", el.GetKind()).
		Str("name", el.GetName()).
		Str("namespace", el.GetNamespace()).
		Msg("received update event")

	er.handler.Handle(el.DeepCopy())
}

func (er *Dynamic) OnDelete(obj interface{}) {
	if !er.log.Debug().Enabled() {
		return
	}

	el := obj.(*unstructured.Unstructured)

	er.log.Debug().
		Str("kind", el.GetKind()).
		Str("name", el.GetName()).
		Str("namespace", el.GetNamespace()).
		Msg("received delete event")
}
