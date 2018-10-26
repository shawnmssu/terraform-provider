---
layout: "ucloud"
page_title: "UCloud: ucloud_lb_attachment"
sidebar_current: "docs-ucloud-resource-lb-attachment"
description: |-
  Provides a Load Balancer Attachment resource for attachment Load Balancer to UHost Instance, etc.
---

# ucloud_lb_attachment

Provides a Load Balancer Attachment resource for attachment Load Balancer to UHost Instance, etc.

## Example Usage

```hcl
resource "ucloud_lb" "web" {
    name = "tf-example-lb"
    tag  = "tf-example"
}

resource "ucloud_lb_listener" "default" {
    load_balancer_id = "${ucloud_lb.web.id}"
    protocol         = "HTTPS"
}

resource "ucloud_security_group" "default" {
    name = "tf-example-eip"
    tag  = "tf-example"

    rules {
        port_range = "80"
        protocol   = "TCP"
        cidr_block = "192.168.0.0/16"
        policy     = "ACCEPT"
    }
}

resource "ucloud_instance" "web" {
    instance_type     = "n-standard-1"
    availability_zone = "cn-sh2-02"

    root_password      = "wA1234567"
    image_id           = "uimage-of3pac"
    security_group     = "${ucloud_security_group.default.id}"

    name              = "tf-example-lb"
    tag               = "tf-example"
}

resource "ucloud_lb_attachment" "example" {
    load_balancer_id = "${ucloud_lb.web.id}"
    listener_id      = "${ucloud_lb_listener.default.id}"
    server_ids       = ["${ucloud_instance.web.id}"]
    server_type      = "instance"
    port             = 80
    enabled          = 1
}
```

## Argument Reference

The following arguments are supported:

* `load_balancer_id` - (Required) The ID of load balancer instance.
* `listener_id` - (Required) The ID of listener servers.
* `server_ids` - (Required) The ID of backend servers.
* `server_type` - (Optional) The types of backend servers, possible values are: "instance" as computing host, "UPM" as physical sever, "UDHost" as dedicated server, "UDocker" as docker host.
* `port` - (Optional) Port opened on the backend server to receive requests, range from 1 to 65535, and default is 80.
* `enabled` - (Optional) The switch of the backend servers, possible values are: 0 as disabled, 1 as enabled, and default is 1.
