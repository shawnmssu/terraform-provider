//Code is generated by ucloud code generator, don't modify it by hand, it will cause undefined behaviors.
//go:generate ucloud-gen-go-api UMem ResizeUDredisSpace

package umem

import (
	"github.com/ucloud/ucloud-sdk-go/ucloud/request"
	"github.com/ucloud/ucloud-sdk-go/ucloud/response"
)

// ResizeUDredisSpaceRequest is request schema for ResizeUDredisSpace action
type ResizeUDredisSpaceRequest struct {
	request.CommonBase

	// 可用区。参见 [可用区列表](../summary/regionlist.html)
	Zone *string `required:"false"`

	// 高性能UMem 内存空间Id
	SpaceId *string `required:"true"`

	// 内存大小, 单位:GB (需要大于原size,<= 1024)
	Size *int `required:"true"`

	// 使用的代金券Id
	CouponId *string `required:"false"`
}

// ResizeUDredisSpaceResponse is response schema for ResizeUDredisSpace action
type ResizeUDredisSpaceResponse struct {
	response.CommonBase
}

// NewResizeUDredisSpaceRequest will create request of ResizeUDredisSpace action.
func (c *UMemClient) NewResizeUDredisSpaceRequest() *ResizeUDredisSpaceRequest {
	req := &ResizeUDredisSpaceRequest{}

	// setup request with client config
	c.client.SetupRequest(req)

	// setup retryable with default retry policy (retry for non-create action and common error)
	req.SetRetryable(true)
	return req
}

// ResizeUDredisSpace - 调整内存空间容量
func (c *UMemClient) ResizeUDredisSpace(req *ResizeUDredisSpaceRequest) (*ResizeUDredisSpaceResponse, error) {
	var err error
	var res ResizeUDredisSpaceResponse

	err = c.client.InvokeAction("ResizeUDredisSpace", req, &res)
	if err != nil {
		return &res, err
	}

	return &res, nil
}