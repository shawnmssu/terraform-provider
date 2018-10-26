package ucloud

import (
	"fmt"

	"github.com/ucloud/ucloud-sdk-go/services/ulb"
	"github.com/ucloud/ucloud-sdk-go/ucloud"
	uerr "github.com/ucloud/ucloud-sdk-go/ucloud/error"
)

func (client *UCloudClient) describeLBById(lbId string) (*ulb.ULBSet, error) {
	conn := client.ulbconn
	req := conn.NewDescribeULBRequest()
	req.ULBId = ucloud.String(lbId)

	resp, err := conn.DescribeULB(req)

	// [API-STYLE] lb api has not found err code, but others don't have
	// TODO: don't use magic number
	if err != nil {
		if uErr, ok := err.(uerr.Error); ok && (uErr.Code() == 4103 || uErr.Code() == 4086) {
			return nil, newNotFoundError(getNotFoundMessage("lb", lbId))
		}
		return nil, err
	}

	if len(resp.DataSet) < 1 {
		return nil, newNotFoundError(getNotFoundMessage("lb", lbId))
	}

	return &resp.DataSet[0], nil
}

func (client *UCloudClient) describeVServerById(lbId, listenerId string) (*ulb.ULBVServerSet, error) {
	conn := client.ulbconn
	req := conn.NewDescribeVServerRequest()
	req.ULBId = ucloud.String(lbId)
	req.VServerId = ucloud.String(listenerId)

	resp, err := conn.DescribeVServer(req)

	// [API-STYLE] vserver api has not found err code, but others don't have
	// TODO: don't use magic number
	if err != nil {
		if uErr, ok := err.(uerr.Error); ok && uErr.Code() == 4103 {
			return nil, newNotFoundError(getNotFoundMessage("listener", listenerId))
		}
		return nil, err
	}

	if len(resp.DataSet) < 1 {
		return nil, newNotFoundError(getNotFoundMessage("listener", listenerId))
	}

	return &resp.DataSet[0], nil
}

func (client *UCloudClient) describeBackendById(lbId, listenerId, backendId string) (*ulb.ULBBackendSet, error) {
	vserverSet, err := client.describeVServerById(lbId, listenerId)

	if err != nil {
		return nil, err
	}

	for i := 0; i < len(vserverSet.BackendSet); i++ {
		backend := vserverSet.BackendSet[i]
		if backend.BackendId == backendId {
			return &backend, nil
		}
	}

	return nil, newNotFoundError(getNotFoundMessage("backend", backendId))
}

func (client *UCloudClient) describePolicyById(lbId, listenerId, policyId string) (*ulb.ULBPolicySet, error) {
	vserverSet, err := client.describeVServerById(lbId, listenerId)

	if err != nil {
		return nil, err
	}

	for i := 0; i < len(vserverSet.PolicySet); i++ {
		policy := vserverSet.PolicySet[i]
		if policy.PolicyId == policyId {
			return &policy, nil
		}
	}

	return nil, newNotFoundError(getNotFoundMessage("policy", policyId))
}

func (client *UCloudClient) removeULBAttachmentBatch(ids []string, lbId string, listenerId string, port, enabled int) error {
	conn := client.ulbconn

	backendIds, err := client.getAvaliableBackendIdsByUHostIds(lbId, listenerId, ids, port)
	if err != nil {
		return fmt.Errorf("%s", err)
	}

	for _, backendId := range backendIds {
		req := conn.NewReleaseBackendRequest()
		req.ULBId = ucloud.String(lbId)
		req.BackendId = ucloud.String(backendId)

		_, err := conn.ReleaseBackend(req)
		if err != nil {
			return err
		}
	}

	return nil
}

func (client *UCloudClient) addULBAttachmentBatch(ids []string, resourceType string, ulbId, listenerId string, port, enabled int) ([]string, error) {
	conn := client.ulbconn

	instances, err := client.describeInstanceByIds(ids)
	if err != nil {
		return nil, err
	}

	backends := []string{}
	for _, inst := range instances {
		id := inst.UHostId

		ip := ""
		for _, ipset := range inst.IPSet {
			if ipset.Type == "Private" {
				ip = ipset.IP
			}
		}

		if ip == "" {
			return nil, fmt.Errorf("no private ip for uhost %s")
		}

		backends = append(backends, fmt.Sprintf("%s|%s|%s|%s|%s", id, resourceType, port, enabled, ip))
	}

	// allocate backend batch
	req := conn.NewAllocateBackendBatchRequest()
	req.ULBId = ucloud.String(ulbId)
	req.VServerId = ucloud.String(listenerId)
	req.Backends = backends

	resp, err := conn.AllocateBackendBatch(req)
	if err != nil {
		return nil, err
	}

	backendIds := []string{}
	for _, backend := range resp.BackendSet {
		backendIds = append(backendIds, backend.BackendId)
	}

	return backendIds, nil
}

func (client *UCloudClient) updateULBAttachmentBatch() error {
	return nil
}

func (client *UCloudClient) getAvaliableBackendIdsByUHostIds(lbId, listenerId string, instIds []string, port int) ([]string, error) {
	vserver, err := client.describeVServerById(lbId, listenerId)
	if err != nil {
		return nil, err
	}

	instMap := map[string]struct{}{}
	for _, instId := range instIds {
		instMap[instId] = struct{}{}
	}

	backendIds := []string{}
	for _, item := range vserver.BackendSet {
		if _, ok := instMap[item.ResourceId]; ok && item.Port == port {
			backendIds = append(backendIds, item.BackendId)
		}
	}

	return backendIds, nil
}
