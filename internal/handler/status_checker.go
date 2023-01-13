package handler

import (
	"strings"

	"github.com/krateoplatformops/capi-watcher/internal/watcher"
	"github.com/rs/zerolog"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	clusterapiv1beta1 "sigs.k8s.io/cluster-api/api/v1beta1"
)

type StatusCheckerOpts struct {
	Log             zerolog.Logger
	ConditionName   string
	ConditionStatus string
	Recorder        record.EventRecorder
}

func NewStatusChecker(o StatusCheckerOpts) watcher.UnstructuredHandler {
	return &statusChecker{
		log:             o.Log,
		conditionName:   o.ConditionName,
		conditionStatus: o.ConditionStatus,
		recorder:        o.Recorder,
	}
}

type statusChecker struct {
	log             zerolog.Logger
	conditionName   string
	conditionStatus string
	recorder        record.EventRecorder
}

func (h *statusChecker) Handle(obj *unstructured.Unstructured) {
	ok, err := h.checkCondition(obj)
	if err != nil {
		h.log.Err(err).
			Str("kind", obj.GetKind()).
			Str("apiVersion", obj.GetAPIVersion()).
			Str("name", obj.GetName()).
			Msg("checking for status.conditions")
	}
	if !ok {
		return
	}

	h.log.Info().
		Str("kind", obj.GetKind()).
		Str("apiVersion", obj.GetAPIVersion()).
		Str("name", obj.GetName()).
		Msgf("%s is %s", h.conditionName, h.conditionStatus)

	var cluster clusterapiv1beta1.Cluster
	err = runtime.DefaultUnstructuredConverter.FromUnstructured(obj.UnstructuredContent(), &cluster)
	if err != nil {
		h.log.Err(err).
			Str("kind", obj.GetKind()).
			Str("apiVersion", obj.GetAPIVersion()).
			Str("name", obj.GetName()).
			Msg("converting from unstructured")
	}

	h.recorder.Eventf(&cluster, corev1.EventTypeNormal, "ClusterReady", "Cluster %s is Ready", obj.GetName())
}

func (h *statusChecker) checkCondition(obj *unstructured.Unstructured) (bool, error) {
	conditions, ok, err := unstructured.NestedSlice(obj.Object, "status", "conditions")
	if err != nil {
		h.log.Err(err).Msg("Looking for status.conditions")
		return false, err
	}
	if !ok {
		h.log.Info().Msg("Object miss status.conditions fields.")
		return false, nil
	}

	for _, el := range conditions {
		co := el.(map[string]interface{})
		name, found, err := unstructured.NestedString(co, "type")
		if !found || err != nil || !strings.EqualFold(name, h.conditionName) {
			continue
		}

		status, found, err := unstructured.NestedString(co, "status")
		if !found || err != nil {
			continue
		}

		return strings.EqualFold(status, h.conditionStatus), nil
	}

	return false, nil
}
