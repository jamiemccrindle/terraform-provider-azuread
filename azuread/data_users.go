package azuread

import (
	"crypto/sha1"
	"encoding/base64"
	"fmt"
	"strings"

	"github.com/Azure/azure-sdk-for-go/services/graphrbac/1.6/graphrbac"
	"github.com/hashicorp/terraform-plugin-sdk/helper/schema"

	"github.com/terraform-providers/terraform-provider-azuread/azuread/helpers/ar"
	"github.com/terraform-providers/terraform-provider-azuread/azuread/helpers/graph"
	"github.com/terraform-providers/terraform-provider-azuread/azuread/helpers/validate"
)

func dataUsers() *schema.Resource {
	return &schema.Resource{
		Read: dataSourceUsersRead,

		Importer: &schema.ResourceImporter{
			State: schema.ImportStatePassthrough,
		},

		Schema: map[string]*schema.Schema{
			"object_ids": {
				Type:         schema.TypeList,
				Optional:     true,
				Computed:     true,
				ExactlyOneOf: []string{"object_ids", "user_principal_names", "mail_nicknames"},
				Elem: &schema.Schema{
					Type:         schema.TypeString,
					ValidateFunc: validate.UUID,
				},
			},

			"user_principal_names": {
				Type:         schema.TypeList,
				Optional:     true,
				Computed:     true,
				ExactlyOneOf: []string{"object_ids", "user_principal_names", "mail_nicknames"},
				Elem: &schema.Schema{
					Type:         schema.TypeString,
					ValidateFunc: validate.NoEmptyStrings,
				},
			},

			"mail_nicknames": {
				Type:         schema.TypeList,
				Optional:     true,
				Computed:     true,
				ExactlyOneOf: []string{"object_ids", "user_principal_names", "mail_nicknames"},
				Elem: &schema.Schema{
					Type:         schema.TypeString,
					ValidateFunc: validate.NoEmptyStrings,
				},
			},

			"ignore_missing": {
				Type:     schema.TypeBool,
				Optional: true,
				Default:  false,
			},

			"users": {
				Type:     schema.TypeList,
				Computed: true,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"account_enabled": {
							Type:     schema.TypeBool,
							Computed: true,
						},

						"display_name": {
							Type:     schema.TypeString,
							Computed: true,
						},

						"immutable_id": {
							Type:     schema.TypeString,
							Computed: true,
						},

						"mail": {
							Type:     schema.TypeString,
							Computed: true,
						},

						"mail_nickname": {
							Type:     schema.TypeString,
							Computed: true,
						},

						"object_id": {
							Type:     schema.TypeString,
							Computed: true,
						},

						"onpremises_sam_account_name": {
							Type:     schema.TypeString,
							Computed: true,
						},

						"onpremises_user_principal_name": {
							Type:     schema.TypeString,
							Computed: true,
						},

						"usage_location": {
							Type:     schema.TypeString,
							Computed: true,
						},

						"user_principal_name": {
							Type:     schema.TypeString,
							Computed: true,
						},
					},
				},
			},
		},
	}
}

func dataSourceUsersRead(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*ArmClient).usersClient
	ctx := meta.(*ArmClient).StopContext

	var users []*graphrbac.User
	expectedCount := 0

	ignoreMissing := d.Get("ignore_missing").(bool)
	if upns, ok := d.Get("user_principal_names").([]interface{}); ok && len(upns) > 0 {
		expectedCount = len(upns)
		for _, v := range upns {
			u, err := client.Get(ctx, v.(string))
			if err != nil {
				if ignoreMissing && ar.ResponseWasNotFound(u.Response) {
					break
				}
				return fmt.Errorf("Error making Read request on AzureAD User with ID %q: %+v", v.(string), err)
			}
			users = append(users, &u)
		}
	} else if oids, ok := d.Get("object_ids").([]interface{}); ok && len(oids) > 0 {
		expectedCount = len(oids)
		for _, v := range oids {
			u, err := graph.UserGetByObjectId(&client, ctx, v.(string))
			if err != nil {
				if ignoreMissing && ar.ResponseWasNotFound(u.Response) {
					break
				}
				return fmt.Errorf("Error finding Azure AD User with object ID %q: %+v", v.(string), err)
			}
			users = append(users, u)
		}
	} else if mailNicknames, ok := d.Get("mail_nicknames").([]interface{}); ok && len(mailNicknames) > 0 {
		expectedCount = len(mailNicknames)
		for _, v := range mailNicknames {
			u, err := graph.UserGetByMailNickname(&client, ctx, v.(string))
			if err != nil {
				if ignoreMissing && ar.ResponseWasNotFound(u.Response) {
					break
				}
				return fmt.Errorf("Error finding Azure AD User with email alias %q: %+v", v.(string), err)
			}
			users = append(users, u)
		}
	}

	if !ignoreMissing && len(users) != expectedCount {
		return fmt.Errorf("Unexpected number of users returned (%d != %d)", len(users), expectedCount)
	}

	if ignoreMissing && len(users) == 0 {
		return fmt.Errorf("No users were returned")
	}

	upns := make([]string, 0, len(users))
	oids := make([]string, 0, len(users))
	mailNicknames := make([]string, 0, len(users))
	var userList []map[string]interface{}
	for _, u := range users {
		if u.ObjectID == nil || u.UserPrincipalName == nil {
			return fmt.Errorf("User with nil ObjectId or UPN was found: %v", u)
		}

		if u.ObjectID != nil || u.UserPrincipalName != nil {
			oids = append(oids, *u.ObjectID)
			upns = append(upns, *u.UserPrincipalName)
			mailNicknames = append(mailNicknames, *u.MailNickname)

			user := make(map[string]interface{})
			user["account_enabled"] = u.AccountEnabled
			user["display_name"] = u.DisplayName
			user["immutable_id"] = u.ImmutableID
			user["mail"] = u.Mail
			user["mail_nickname"] = u.MailNickname
			user["object_id"] = u.ObjectID
			user["onpremises_sam_account_name"] = u.AdditionalProperties["onPremisesSamAccountName"]
			user["onpremises_user_principal_name"] = u.AdditionalProperties["onPremisesUserPrincipalName"]
			user["usage_location"] = u.UsageLocation
			user["user_principal_name"] = u.UserPrincipalName
			userList = append(userList, user)
		}
	}

	h := sha1.New()
	if _, err := h.Write([]byte(strings.Join(upns, "-"))); err != nil {
		return fmt.Errorf("Unable to compute hash for upns: %v", err)
	}

	d.SetId("users#" + base64.URLEncoding.EncodeToString(h.Sum(nil)))
	d.Set("object_ids", oids)
	d.Set("user_principal_names", upns)
	d.Set("mail_nicknames", mailNicknames)
	d.Set("users", userList)

	return nil
}
