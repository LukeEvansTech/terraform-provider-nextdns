package nextdns

import "github.com/hashicorp/terraform-plugin-testing/terraform"

// Short aliases keep the check functions readable.
type (
	terraformState = terraform.State
	instanceState  = terraform.InstanceState
)
