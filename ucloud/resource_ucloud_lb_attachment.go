package ucloud

import (
	"fmt"
	"time"

	"github.com/hashicorp/terraform/helper/resource"
	"github.com/hashicorp/terraform/helper/schema"
	"github.com/ucloud/ucloud-sdk-go/ucloud"
)

func resourceUCloudLBAttachment() *schema.Resource {
	return &schema.Resource{
		Create: resourceUCloudLBAttachmentCreate,
		Read:   resourceUCloudLBAttachmentRead,
		Update: resourceUCloudLBAttachmentUpdate,
		Delete: resourceUCloudLBAttachmentDelete,

		Schema: map[string]*schema.Schema{
			"load_balancer_id": &schema.Schema{
				Type:     schema.TypeString,
				Required: true,
				ForceNew: true,
			},

			"listener_id": &schema.Schema{
				Type:     schema.TypeString,
				Required: true,
				ForceNew: true,
			},

			"instance_servers": &schema.Schema{
				Type:     schema.TypeSet,
				Optional: true,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"server_ids": &schema.Schema{
							Type:     schema.TypeList,
							Required: true,
							Elem:     &schema.Schema{Type: schema.TypeString},
							MaxItems: 20,
							MinItems: 1,
						},

						"port": &schema.Schema{
							Type:         schema.TypeInt,
							Required:     true,
							ValidateFunc: validateIntegerInRange(1, 65535),
						},

						"enabled": &schema.Schema{
							Type:         schema.TypeInt,
							Optional:     true,
							Default:      1,
							ValidateFunc: validateIntegerInRange(0, 1),
						},
					},
				},
				MaxItems: 20,
				MinItems: 0,
			},

			"status": &schema.Schema{
				Type:     schema.TypeString,
				Computed: true,
			},
		},
	}
}

func resourceUCloudLBAttachmentCreate(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*UCloudClient)
	conn := client.ulbconn

	lbId := d.Get("load_balancer_id").(string)
	listenerId := d.Get("listener_id").(string)

	instanceIds, err := buildBackendParameter(d.Get("instance_servers"), "UHost", meta)

	req := conn.NewAllocateBackendBatchRequest()
	req.ULBId = ucloud.String(lbId)
	req.VServerId = ucloud.String(listenerId)
	req.Backends = instanceIds

	resp, err := conn.AllocateBackendBatch(req)
	if err != nil {
		return fmt.Errorf("error in create lb attachment, %s", err)
	}

	// lbAttachmentIds := []string{}
	// for _, item := range resp.backendSet {
	// 	lbAttachmentIds = append(lbAttachmentIds, item.BackendId)
	// }

	// lbAttachmentId := strings.Join(lbAttachmentIds, "-")
	// d.SetId(lbAttachmentId)

	// after create lb attachment, we need to wait it initialized
	stateConf := &resource.StateChangeConf{
		Pending:    []string{"pending"},
		Target:     []string{"initialized"},
		Timeout:    10 * time.Minute,
		Delay:      5 * time.Second,
		MinTimeout: 3 * time.Second,
		Refresh: func() (interface{}, string, error) {
			backendSet, err := client.describeBackendById(lbId, listenerId, d.Id())
			if err != nil {
				if isNotFoundError(err) {
					return nil, "pending", nil
				}
				return nil, "", err
			}

			state := lbAttachmentStatus.transform(backendSet.Status)
			if state != "normalRunning" {
				state = "pending"
			} else {
				state = "initialized"
			}

			return backendSet, state, nil
		},
	}
	_, err = stateConf.WaitForState()

	if err != nil {
		return fmt.Errorf("wait for lb attachment initialize failed in create lb attachment %s, %s", d.Id(), err)
	}

	return resourceUCloudLBAttachmentUpdate(d, meta)
}

func resourceUCloudLBAttachmentUpdate(d *schema.ResourceData, meta interface{}) error {
	conn := meta.(*UCloudClient).ulbconn
	d.Partial(true)

	isChanged := false
	ulbId := d.Get("load_balancer_id").(string)
	listenerId := d.Get("listener_id").(string)
	req := conn.NewUpdateBackendAttributeRequest()
	req.ULBId = ucloud.String(ulbId)
	req.BackendId = ucloud.String(d.Id())

	if d.HasChange("instance_servers") && !d.IsNewResource() {
		old, new := d.GetChange("instance_servers")
		os := old.(*schema.Set)
		ns := new.(*schema.Set)

		addSet, removeSet, updateSet := buildChangeItems(os, ns)

		if len(removeSet) > 0 {
			backendIds, _, err := getAvaliableBackendIdsByUHostIds(meta, getServersIds(removeSet), ulbId, listenerId)
			if err != nil {
				return err // TODO
			}

			for _, id := range backendIds {
				req := conn.NewReleaseBackendRequest()
				req.ULBId = ucloud.String(ulbId)
				req.BackendId = ucloud.String(id)
				_, err := conn.ReleaseBackend(req)
				if err != nil {
					return err // TODO
				}
			}
		}

		if len(addSet) > 0 {
			for _, item := range addSet {
				req := conn.NewAllocateBackendRequest()
				req.ULBId = ucloud.String(ulbId)
				req.ResourceType = ucloud.String("UHost")
				req.ResourceId = ucloud.String(item.uhostId)
				_, err := conn.AllocateBackend(req)
				if err != nil {
					return err // TODO
				}
			}
			req := conn.NewAllocateBackendRequest()
		}

		if len(updateSet) > 0 {
			backendIds, portMap, err := getAvaliableBackendIdsByUHostIds(meta, getServersIds(updateSet), ulbId, listenerId)
			if err != nil {
				return err // TODO
			}

			var resourceId string
			for _, backendId := range backendIds {
				if v, ok := portMap[backendId]; ok {
					req := conn.NewUpdateBackendAttributeRequest()
					req.ULBId = ucloud.String(ulbId)
					req.BackendId = ucloud.String(v)

					req.Port = ucloud.int()
					resourceId = v
				}

				req := conn.NewUpdateBackendAttributeRequest()
				req.ULBId = ucloud.String(ulbId)
				req.BackendId = ucloud.String(backendId)
				req.Port = ucloud.int()
				_, err := conn.ReleaseBackend(req)
				if err != nil {
					return err // TODO
				}
			}

			req := conn.NewUpdateBackendAttributeRequest()
		}

		d.SetPartial("servers")
	}

	d.Partial(false)

	return resourceUCloudLBAttachmentRead(d, meta)
}

type changedServer struct {
	uhostId string
	port    int
	enabled int
}

func buildChangeItems(os *schema.Set, ns *schema.Set) ([]changedServer, []changedServer, []changedServer) {
	oldServers := expandAllServers(os)
	newServers := expandAllServers(ns)

	oldMap := map[string]changedServer{}
	for i := 0; i < len(oldServers); i++ {
		item := oldServers[i]
		oldMap[item.uhostId] = item
	}

	newMap := map[string]changedServer{}
	for i := 0; i < len(newServers); i++ {
		item := newServers[i]
		newMap[item.uhostId] = item
	}

	removeServers := []changedServer{}
	for i := 0; i < len(oldServers); i++ {
		oldItem := oldServers[i]
		_, ok := newMap[oldItem.uhostId]
		if !ok {
			removeServers = append(removeServers, oldItem)
		}
	}

	addServers := []changedServer{}
	updateServers := []changedServer{}
	for i := 0; i < len(newServers); i++ {
		newItem := newServers[i]
		oldItem, ok := oldMap[newItem.uhostId]
		if !ok {
			addServers = append(addServers, newItem)
		}

		if ok && (oldItem.port != newItem.port || oldItem.enabled != newItem.enabled) {
			updateServers = append(updateServers, newItem)
		}
	}

	return addServers, removeServers, updateServers
}

func getServersIds(servers []changedServer) []string {
	ids := make([]string, len(servers))
	for i := 0; i < len(servers); i++ {
		ids[i] = servers[i].uhostId
	}
	return ids
}

func expandAllServers(set *schema.Set) []changedServer {
	var servers []changedServer
	for _, item := range set.List() {
		s := item.(map[string]interface{})
		for _, id := range ifaceToStringSlice(s["server_ids"]) {
			c := changedServer{
				uhostId: id,
				port:    s["port"].(int),
				enabled: s["enbaled"].(int),
			}
			servers = append(servers, c)
		}
	}
	return servers
}

func resourceUCloudLBAttachmentRead(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*UCloudClient)

	lbId := d.Get("load_balancer_id").(string)
	listenerId := d.Get("listener_id").(string)

	backendSet, err := client.describeBackendById(lbId, listenerId, d.Id())

	if err != nil {
		if isNotFoundError(err) {
			d.SetId("")
			return nil
		}
		return fmt.Errorf("do %s failed in read lb attachment %s, %s", "DescribeVServer", d.Id(), err)
	}

	d.Set("resource_id", backendSet.ResourceId)
	d.Set("resource_type", uHostMap.unconvert(backendSet.ResourceType))
	d.Set("port", backendSet.Port)
	d.Set("private_ip", backendSet.PrivateIP)
	d.Set("status", lbAttachmentStatus.transform(backendSet.Status))

	return nil
}

func resourceUCloudLBAttachmentDelete(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*UCloudClient)
	conn := client.ulbconn

	lbId := d.Get("load_balancer_id").(string)
	listenerId := d.Get("listener_id").(string)

	req := conn.NewReleaseBackendRequest()
	req.ULBId = ucloud.String(lbId)
	req.BackendId = ucloud.String(d.Id())

	return resource.Retry(5*time.Minute, func() *resource.RetryError {

		if _, err := conn.ReleaseBackend(req); err != nil {
			return resource.NonRetryableError(fmt.Errorf("error in delete lb attachment %s, %s", d.Id(), err))
		}

		_, err := client.describeBackendById(lbId, listenerId, d.Id())

		if err != nil {
			if isNotFoundError(err) {
				return nil
			}
			return resource.NonRetryableError(fmt.Errorf("do %s failed in delete lb attachment %s, %s", "DescribeVServer", d.Id(), err))
		}

		return resource.RetryableError(fmt.Errorf("delete lb attachment but it still exists"))
	})
}

func getAvaliableBackendIdsByUHostIds(meta interface{}, uhostIds []string, lbId, listenerId string) ([]string, map[string]string, error) {
	client := meta.(*UCloudClient)

	vserver, err := client.describeVServerById(lbId, listenerId)
	if err != nil {
		return nil, nil, err
	}

	idMap := map[string]string{}
	portMap := map[string]string{}
	for _, item := range vserver.BackendSet {
		idMap[item.ResourceId] = item.BackendId
		portMap[item.BackendId] = item.ResourceId
	}

	backendIds := []string{}
	for _, uhostId := range uhostIds {
		if v, ok := idMap[uhostId]; ok {
			backendIds = append(backendIds, v)
		}
	}

	return backendIds, portMap, nil
}

func buildBackendParameter(iface interface{}, resourceType string, meta interface{}) ([]string, error) {
	client := meta.(*UCloudClient)
	backends := []string{}
	for _, item := range iface.(*schema.Set).List() {
		backend := item.(map[string]interface{})
		ids := ifaceToStringSlice(backend["server_ids"].([]interface{}))
		instances, err := client.describeInstanceByIds(ids)
		if err != nil {
			return nil, fmt.Errorf("do %s failed in read lb attachment, %s", "DescribeUHostInstance", err)
		}

		for _, instance := range instances {
			id := instance.UHostId
			var privateIp string
			for _, ipset := range instance.IPSet {
				if ipset.Type == "Private" {
					privateIp = ipset.IP
				}
			}

			s := fmt.Sprintf("%s|%s|%s|%s|%s", id, resourceType, backend["port"], backend["enabled"], privateIp)
			backends = append(backends, s)
		}
	}
	return backends, nil
}
