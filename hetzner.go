package main

import (
	"context"
	"fmt"

	"github.com/diamondburned/arikawa/v3/api"
	"github.com/diamondburned/arikawa/v3/api/cmdroute"
	"github.com/diamondburned/arikawa/v3/discord"
	"github.com/hetznercloud/hcloud-go/hcloud"
)

func (c *clouds) spawnResponseHetzner(ctx context.Context, data cmdroute.ComponentData) *api.InteractionResponse {
	labels := make(map[string]string)
	labels["channel"] = data.Event.ChannelID.String()
	labels["guild"] = data.Event.GuildID.String()
	region := data.Event.Message.Components.Find("region").(*discord.StringSelectComponent).Placeholder
	opts := hcloud.ServerCreateOpts{
		Name:             data.Event.Channel.Name,
		ServerType:       &hcloud.ServerType{ID: 104, Name: "CX22"},
		Image:            &hcloud.Image{Name: "ubuntu-24.04"},
		SSHKeys:          nil,
		Location:         &hcloud.Location{Country: region},
		StartAfterCreate: BoolPointer(true),
		Labels:           labels,
		Networks:         nil,
	}
	res, _, err := c.Hetzner.Server.Create(ctx, opts)
	if err != nil {
		panic(err.Error())
	}
	return c.spawnResponse(res.Server.PublicNet.IPv4.IP.String(), res.RootPassword, "root", "Hetzner enforces changing the root password, please post what you change it to.", "hetzner")
}

func (c *clouds) deleteHetzner(ctx context.Context, data cmdroute.ComponentData) *api.InteractionResponse {
	listOpts := hcloud.ServerListOpts{
		ListOpts: hcloud.ListOpts{
			LabelSelector: fmt.Sprintf("channel=%s,guild=%s", data.Event.ChannelID.String(), data.Event.GuildID.String()),
		},
	}
	listRes, _, err := c.Hetzner.Server.List(ctx, listOpts)
	if err != nil {
		panic(err)
	}
	for _, server := range listRes {
		_, _, err := c.Hetzner.Server.DeleteWithResult(ctx, server)
		if err != nil {
			panic(err)
		}
	}
	return c.deleteResponse()
}
