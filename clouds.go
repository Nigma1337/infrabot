package main

import (
	"context"
	"fmt"

	"github.com/diamondburned/arikawa/v3/api"
	"github.com/diamondburned/arikawa/v3/discord"
	"github.com/diamondburned/arikawa/v3/utils/json/option"
	"github.com/hetznercloud/hcloud-go/hcloud"
)

type clouds struct {
	Hetzner   *hcloud.Client
	Aws       awooga
	Avaliable []string
}

func (c *clouds) getCloudOpts(selected string) []discord.SelectOption {
	var res []discord.SelectOption
	for _, cloud := range c.Avaliable {
		if cloud == selected {
			res = append(res, discord.SelectOption{Label: cloud, Value: cloud, Default: true})
		} else {
			res = append(res, discord.SelectOption{Label: cloud, Value: cloud})
		}
	}
	return res
}

func (c *clouds) getCloudRegions(cloud string, region string) []discord.SelectOption {
	var res []discord.SelectOption
	ctx := context.Background()
	if cloud == "" {
		cloud = "hetzner"
	}
	switch cloud {
	case "hetzner":
		locations, err := c.Hetzner.Location.All(ctx)
		if err != nil {
			panic("bad")
		}
		for _, location := range locations {
			if region == location.Name {
				res = append(res, discord.SelectOption{Label: location.City, Value: location.Name, Default: true})
			} else {
				res = append(res, discord.SelectOption{Label: location.City, Value: location.Name})
			}
		}
	case "aws":
		locations, err := c.Aws.DescribeRegions(ctx, nil)
		if err != nil {
			panic("bad")
		}
		for _, location := range locations.Regions {
			if region == *location.RegionName {
				res = append(res, discord.SelectOption{Label: *location.RegionName, Value: *location.RegionName, Default: true})
			} else {
				res = append(res, discord.SelectOption{Label: *location.RegionName, Value: *location.RegionName})
			}
		}
	}
	return res
}

func (c *clouds) getInstanceTypes(cloud string, instanceType string) []discord.SelectOption {
	var res []discord.SelectOption
	switch cloud {
	case "hetzner":
		// Hetzner only gets CX22.
		res = append(res, discord.SelectOption{Label: "CX22", Value: "CX22", Default: true})
	case "aws":
		// AWS instance types can vary widely, so we will just return a few common ones.
		awsTypes := []struct {
			Name  string
			Label string
		}{
			// General purpose instances
			{"t3.small", "t3.small (2 vCPU, 2 GiB RAM)"},
			{"t3.medium", "t3.medium (2 vCPU, 4 GiB RAM)"},
			{"t3.large", "t3.large (2 vCPU, 8 GiB RAM)"},
			{"t3.xlarge", "t3.xlarge (4 vCPU, 16 GiB RAM)"},
			// Compute optimized instances
			{"C7i.large", "C7i.large (2 vCPU, 4 GiB RAM)"},
			{"C7i.xlarge", "C7i.xlarge (4 vCPU, 8 GiB RAM)"},
			// Memory optimized instances
			{"R7i.large", "R7i.large (2 vCPU, 16 GiB RAM)"},
			{"R7i.xlarge", "R7i.xlarge (4 vCPU, 32 GiB RAM)"},
		}
		for _, t := range awsTypes {
			if instanceType == t.Name {
				res = append(res, discord.SelectOption{Label: t.Label, Value: t.Name, Default: true})
				continue
			} else {
				res = append(res, discord.SelectOption{Label: t.Label, Value: t.Name})
			}
		}
	default:
		// Default case, no specific instance types.
		res = append(res, discord.SelectOption{Label: "None", Value: "None"})
	}
	return res
}

func (c *clouds) spawnResponse(ip string, password string, user string, extraInfo string, cloud string) *api.InteractionResponse {
	if extraInfo != "" {
		extraInfo = "\n\nNote: " + extraInfo
	}
	response := fmt.Sprintf("your new server located at `%s` with username `%s` and password `%s` is now up! %s", ip, user, password, extraInfo)
	return &api.InteractionResponse{
		Type: api.UpdateMessage,
		Data: &api.InteractionResponseData{
			Content: option.NewNullableString(response),
			Components: discord.ComponentsPtr(
				&discord.ActionRowComponent{
					&discord.ButtonComponent{
						Label:    "Destroy",
						CustomID: discord.ComponentID("destroy_" + cloud),
						Style:    discord.DangerButtonStyle(),
					},
				}),
		},
	}
}

func (c *clouds) deleteResponse() *api.InteractionResponse {
	response := "Job done!"
	return &api.InteractionResponse{
		Type: api.UpdateMessage,
		Data: &api.InteractionResponseData{
			Content:    option.NewNullableString(response),
			Components: discord.ComponentsPtr(),
		},
	}
}
