package ucloud

import (
	"github.com/ucloud/ucloud-sdk-go/services/udb"
	"github.com/ucloud/ucloud-sdk-go/ucloud"
)

func (client *UCloudClient) describeDbInstanceById(dbInstanceId string) (*udb.UDBInstanceSet, error) {
	req := client.udbconn.NewDescribeUDBInstanceRequest()
	req.DBId = ucloud.String(dbInstanceId)

	resp, err := client.udbconn.DescribeUDBInstance(req)
	if err != nil {
		return nil, err
	}
	if len(resp.DataSet) < 1 {
		return nil, newNotFoundError(getNotFoundMessage("db_instance", dbInstanceId))
	}

	return &resp.DataSet[0], nil
}
