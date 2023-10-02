package annotations

import (
	nadv1 "github.com/k8snetworkplumbingwg/network-attachment-definition-client/pkg/apis/k8s.cni.cncf.io/v1"
	multusapi "gopkg.in/k8snetworkplumbingwg/multus-cni.v3/pkg/server/api"
)

type AttachmentResult struct {
	attachment *nadv1.NetworkSelectionElement
	result     *multusapi.Response
}

func NewAttachmentResult(attachment *nadv1.NetworkSelectionElement, result *multusapi.Response) *AttachmentResult {
	return &AttachmentResult{
		attachment: attachment,
		result:     result,
	}
}

func (ar *AttachmentResult) IsValid() bool {
	return ar != nil && ar.attachment != nil
}

func (ar *AttachmentResult) HasResult() bool {
	return ar.IsValid() && ar.result != nil
}
