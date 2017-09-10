package openstack

import (
	"fmt"
	"log"
	"time"

	"github.com/gophercloud/gophercloud"
	"github.com/gophercloud/gophercloud/openstack/db/v1/databases"
	"github.com/gophercloud/gophercloud/openstack/db/v1/users"
	"github.com/hashicorp/terraform/helper/resource"
	"github.com/hashicorp/terraform/helper/schema"
)

func resourceDbUser() *schema.Resource {
	return &schema.Resource{
		Create: resourceDbUserCreate,
		Read:   resourceDbUserRead,
		Delete: resourceDbUserDelete,
		Importer: &schema.ResourceImporter{
			State: schema.ImportStatePassthrough,
		},

		Timeouts: &schema.ResourceTimeout{
			Create: schema.DefaultTimeout(10 * time.Minute),
			Delete: schema.DefaultTimeout(10 * time.Minute),
		},

		Schema: map[string]*schema.Schema{
			"region": &schema.Schema{
				Type:        schema.TypeString,
				Required:    true,
				ForceNew:    true,
				DefaultFunc: schema.EnvDefaultFunc("OS_REGION_NAME", ""),
			},
			"name": &schema.Schema{
				Type:     schema.TypeString,
				Required: true,
				ForceNew: true,
			},
			"instance": &schema.Schema{
				Type:     schema.TypeString,
				Required: true,
				ForceNew: true,
			},
			"password": &schema.Schema{
				Type:     schema.TypeString,
				Required: true,
				ForceNew: true,
			},
			"host": &schema.Schema{
				Type:     schema.TypeString,
				Optional: true,
				ForceNew: true,
			},
			"databases": &schema.Schema{
				Type:     schema.TypeSet,
				Optional: true,
				Computed: true,
				Elem:     &schema.Schema{Type: schema.TypeString},
				Set:      schema.HashString,
			},
		},
	}
}

func resourceDbUserCreate(d *schema.ResourceData, meta interface{}) error {
	config := meta.(*Config)
	databaseInstanceClient, err := config.databaseInstanceClient(GetRegion(d, config))
	if err != nil {
		return fmt.Errorf("Error creating cloud database client: %s", err)
	}

	username := d.Get("name").(string)

	raw_dbs := d.Get("databases").(*schema.Set).List()
	var dbs databases.BatchCreateOpts
	for _, db := range raw_dbs {
		dbs = append(dbs, databases.CreateOpts{
			Name: db.(string),
		})
	}

	var users_list users.BatchCreateOpts
	users_list = append(users_list, users.CreateOpts{
		Name:      username,
		Password:  d.Get("password").(string),
		Host:      d.Get("host").(string),
		Databases: dbs,
	})

	instance_id := d.Get("instance").(string)

	users.Create(databaseInstanceClient, instance_id, users_list)

	stateConf := &resource.StateChangeConf{
		Pending:    []string{"BUILD"},
		Target:     []string{"ACTIVE"},
		Refresh:    DbUserStateRefreshFunc(databaseInstanceClient, instance_id, username),
		Timeout:    d.Timeout(schema.TimeoutCreate),
		Delay:      10 * time.Second,
		MinTimeout: 3 * time.Second,
	}

	_, err = stateConf.WaitForState()
	if err != nil {
		return fmt.Errorf(
			"Error waiting for user (%s) to be created", err)
	}

	// Store the ID now
	d.SetId(instance_id)

	return resourceDbUserRead(d, meta)
}

func resourceDbUserRead(d *schema.ResourceData, meta interface{}) error {
	config := meta.(*Config)
	databaseInstanceClient, err := config.databaseInstanceClient(GetRegion(d, config))
	if err != nil {
		return fmt.Errorf("Error creating OpenStack cloud database client: %s", err)
	}

	username := d.Get("name").(string)

	pages, err := users.List(databaseInstanceClient, d.Id()).AllPages()
	if err != nil {
		return fmt.Errorf("Unable to retrieve users, pages: %s", err)
	}
	allUsers, err := users.ExtractUsers(pages)
	if err != nil {
		return fmt.Errorf("Unable to retrieve users, extract: %s", err)
	}

	for _, v := range allUsers {
		if v.Name == username {
			d.Set("name", v.Name)
			d.Set("password", v.Password)
			d.Set("databases", v.Databases)
			break
		}
	}
	log.Printf("[DEBUG] Retrieved user %s", username)

	return nil
}

func resourceDbUserDelete(d *schema.ResourceData, meta interface{}) error {
	config := meta.(*Config)
	databaseInstanceClient, err := config.databaseInstanceClient(GetRegion(d, config))
	if err != nil {
		return fmt.Errorf("Error creating OpenStack cloud database client: %s", err)
	}

	username := d.Get("name").(string)

	pages, err := users.List(databaseInstanceClient, d.Id()).AllPages()
	allUsers, err := users.ExtractUsers(pages)
	if err != nil {
		return fmt.Errorf("Unable to retrieve users: %s", err)
	}

	log.Printf("Retrieved users", allUsers)
	log.Printf("Looking for user", username)

	userExists := false

	for _, v := range allUsers {
		if v.Name == username {
			userExists = true
			break
		}
	}

	if !userExists {
		log.Printf("User %s was not found on instance %s", username, d.Id())
	}

	users.Delete(databaseInstanceClient, d.Id(), username)

	d.SetId("")
	return nil
}

// DbUserStateRefreshFunc returns a resource.StateRefreshFunc that is used to watch db user.
func DbUserStateRefreshFunc(client *gophercloud.ServiceClient, instance_id string, username string) resource.StateRefreshFunc {
	return func() (interface{}, string, error) {

		pages, err := users.List(client, instance_id).AllPages()
		if err != nil {
			return nil, "", fmt.Errorf("Unable to retrieve users, pages: %s", err)
		}

		allUsers, err := users.ExtractUsers(pages)
		if err != nil {
			return nil, "", fmt.Errorf("Unable to retrieve users, extract: %s", err)
		}

		for _, v := range allUsers {
			if v.Name == username {
				return v, "ACTIVE", nil
			}
		}

		return nil, "", fmt.Errorf("Error retrieving user %s status", username)
	}
}
