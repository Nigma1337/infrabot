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
	if len(res) == 0 {
		res = append(res, discord.SelectOption{Label: "nowhere", Value: ""})
	}
	return res
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
