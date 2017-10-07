package openstack

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform/helper/resource"
	"github.com/hashicorp/terraform/terraform"

	"github.com/gophercloud/gophercloud/openstack/db/v1/instances"
)

func TestAccDatabaseInstance_basic(t *testing.T) {
	var instance instances.Instance

	resource.Test(t, resource.TestCase{
		PreCheck:  func() { testAccPreCheck(t) },
		Providers: testAccProviders,
		Steps: []resource.TestStep{
			resource.TestStep{
				Config: testAccDatabaseInstanceBasic,
				Check: resource.ComposeTestCheckFunc(
					testAccCheckDatabaseInstanceExists(
						"openstack_db_instance.basic", &instance),
					resource.TestCheckResourceAttr(
						"openstack_db_instance.basic", "name", "basic"),
				),
			},
		},
	})
}

func testAccCheckDatabaseInstanceExists(n string, instance *instances.Instance) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[n]
		if !ok {
			return fmt.Errorf("Not found: %s", n)
		}

		if rs.Primary.ID == "" {
			return fmt.Errorf("No ID is set")
		}

		config := testAccProvider.Meta().(*Config)
		databaseInstanceClient, err := config.databaseInstanceClient(OS_REGION_NAME)
		if err != nil {
			return fmt.Errorf("Error creating OpenStack compute client: %s", err)
		}

		found, err := instances.Get(databaseInstanceClient, rs.Primary.ID).Extract()
		if err != nil {
			return err
		}

		if found.ID != rs.Primary.ID {
			return fmt.Errorf("Instance not found")
		}

		*instance = *found

		return nil
	}
}

const testAccDatabaseInstanceBasic = `
resource "openstack_db_instance" "basic" {
	name = "basic"
}
`
