package ucloud

import (
	"fmt"
	"strings"
	"time"

	"github.com/hashicorp/terraform/helper/resource"
	"github.com/hashicorp/terraform/helper/schema"
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

			"server_type": &schema.Schema{
				Type:     schema.TypeString,
				Optional: true,
				Default:  "instance",
			},

			"server_ids": &schema.Schema{
				Type:     schema.TypeSet,
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
	}
}

func resourceUCloudLBAttachmentCreate(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*UCloudClient)

	ulbId := d.Get("load_balancer_id").(string)
	listenerId := d.Get("listener_id").(string)
	port := d.Get("port").(int)
	enabled := d.Get("enabled").(int)
	serverType := d.Get("server_type").(string)
	ids := ifaceToStringSlice(d.Get("server_ids").([]interface{}))

	backendIds, err := client.addULBAttachmentBatch(ids, serverType, ulbId, listenerId, port, enabled)
	if err != nil {
		return fmt.Errorf("do %s failed in create ulb attachment, %s", "AllocateBackendBatch", err)
	}

	d.SetId(strings.Join(backendIds, "-"))
	return resourceUCloudLBAttachmentUpdate(d, meta)
}

func resourceUCloudLBAttachmentUpdate(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*UCloudClient)
	conn := client.ulbconn

	d.Partial(true)

	ulbId := d.Get("load_balancer_id").(string)
	listenerId := d.Get("listener_id").(string)
	port := d.Get("port").(int)
	enabled := d.Get("enabled").(int)
	serverType := d.Get("server_type").(string)

	if d.HasChange("server_ids") && !d.IsNewResource() {
		old, new := d.GetChange("server_ids")
		os := old.(*schema.Set)
		ns := new.(*schema.Set)
		add := ns.Difference(os).List()
		remove := os.Difference(ns).List()

		if len(remove) > 0 {
			ids := ifaceToStringSlice(remove)
			err := client.removeULBAttachmentBatch(ids, ulbId, listenerId, port, enabled)
			if err != nil {
				return fmt.Errorf("do %s failed in update ulb attachment, %s", "ReleaseBackend", err)
			}
		}

		if len(add) > 0 {
			ids := ifaceToStringSlice(add)
			if err := client.addULBAttachmentBatch(ids, serverType, ulbId, listenerId, port, enabled); err != nil {
				return fmt.Errorf("do %s failed in update ulb attachment, %s", "AllocateBackendBatch", err)
			}
		}

		d.SetPartial("server_ids")
	}

	isChanged := true

	if d.HasChange("port") && !d.IsNewResource() {
		port := d.Get("port").(int)
	}

	if d.HasChange("enabled") && !d.IsNewResource() {
		enabled := d.Get("enabled").(int)
	}

	err := client.updateULBAttachmentBatch(ids, ulbId, listenerId, port)
	if err != nil {
		return fmt.Errorf("")
	}

	d.Partial(false)

	return resourceUCloudLBAttachmentRead(d, meta)
}

func resourceUCloudLBAttachmentRead(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*UCloudClient)

	lbId := d.Get("load_balancer_id").(string)
	listenerId := d.Get("listener_id").(string)

	listener, err := client.describeVServerById(lbId, listenerId)
	if err != nil {
		return fmt.Errorf("do %s failed in read lb attachment %s, %s", "DescribeVServer", d.Id(), err)
	}

	return nil
}

func resourceUCloudLBAttachmentDelete(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*UCloudClient)

	lbId := d.Get("load_balancer_id").(string)
	listenerId := d.Get("listener_id").(string)
	port := d.Get("port").(int)
	enabled := d.Get("enabled").(int)
	serverIds := ifaceToStringSlice(d.Get("server_ids").([]interface{}))

	return resource.Retry(5*time.Minute, func() *resource.RetryError {
		err := client.removeULBAttachmentBatch(serverIds, lbId, listenerId, port, enabled)
		if err != nil {
			return resource.NonRetryableError(fmt.Errorf("do %s failed in delete attachment %s, %s", "ReleaseBackend", d.Id(), err))
		}

		backendIds, err := client.getAvaliableBackendIdsByUHostIds(lbId, listenerId, serverIds, port)
		if err != nil {
			return resource.NonRetryableError(fmt.Errorf("do %s failed in delete attachment %s, %s", "ReleaseBackend", d.Id(), err))
		}

		if len(backendIds) > 0 {
			return resource.RetryableError(fmt.Errorf("delete lb listener but it still exists"))
		}

		return nil
	})
}

func waitForStateBatch(client *UCloudClient, ids []string, ulbId, listenerId string, port int) error {
	// wait for all backend is allocated
	stateConf := &resource.StateChangeConf{
		Pending:    []string{"pending"},
		Target:     []string{"initialized"},
		Timeout:    10 * time.Minute,
		Delay:      5 * time.Second,
		MinTimeout: 3 * time.Second,
		Refresh: func() (interface{}, string, error) {
			// query backend by listener
			listener, err := client.describeVServerById(ulbId, listenerId)
			if err != nil {
				return nil, "", err
			}

			// construct status map by serverid:port
			instMap := map[string]string{}
			for _, backend := range listener.BackendSet {
				code := fmt.Sprintf("%s:%s", backend.ResourceId, backend.Port)
				instMap[code] = lbAttachmentStatus.transform(backend.Status)
			}

			// wait for all id status is normalRunning
			for _, id := range ids {
				code := fmt.Sprintf("%s:%s", id, port)

				if status, ok := instMap[code]; !ok {
					return nil, "pending", nil
				} else if status != "normalRunning" {
					return nil, "pending", nil
				}
			}

			return listener.BackendSet, "initialized", nil
		},
	}

	_, err := stateConf.WaitForState()
	if err != nil {
		return err
	}

	return nil
}
