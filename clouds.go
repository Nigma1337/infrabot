package main

import (
	"context"
	b64 "encoding/base64"
	"fmt"
	"sort"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/diamondburned/arikawa/v3/api"
	"github.com/diamondburned/arikawa/v3/api/cmdroute"
	"github.com/diamondburned/arikawa/v3/discord"
	"github.com/diamondburned/arikawa/v3/utils/json/option"
	"github.com/hetznercloud/hcloud-go/hcloud"
)

type awooga struct {
	*ec2.Client
	sg string
}

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

func (c *clouds) spawnResponseHetzner(ctx context.Context, data cmdroute.ComponentData) *api.InteractionResponse {
	region := data.Event.Message.Components.Find("region").(*discord.StringSelectComponent).Placeholder
	opts := hcloud.ServerCreateOpts{
		Name:             "awesomeboob123456",
		ServerType:       &hcloud.ServerType{ID: 3, Name: "CX21"},
		Image:            &hcloud.Image{Name: "ubuntu-22.04"},
		SSHKeys:          nil,
		Location:         &hcloud.Location{Country: region},
		StartAfterCreate: BoolPointer(true),
		Labels:           nil,
		Networks:         nil,
	}
	res, _, err := c.Hetzner.Server.Create(ctx, opts)
	if err != nil {
		panic(err.Error())
	}
	return c.spawnResponse(res.Server.PublicNet.IPv4.IP.String(), res.RootPassword)
}

func (c *clouds) spawnResponseAWS(ctx context.Context, data cmdroute.ComponentData) *api.InteractionResponse {
	images, err := c.Aws.DescribeImages(ctx, &ec2.DescribeImagesInput{
		Filters: []types.Filter{
			{
				Name:   aws.String("owner-id"),
				Values: []string{"099720109477"}, // Cannonical ownership id, see https://ubuntu.com/server/docs/cloud-images/amazon-ec2
			},
			{
				Name:   aws.String("creation-date"),
				Values: []string{"2023*"},
			},
			{
				Name:   aws.String("name"),
				Values: []string{"ubuntu/images/hvm-ssd/*22.04*"},
			},
			{
				Name:   aws.String("architecture"),
				Values: []string{"x86_64"},
			},
		},
	},
	)
	if err != nil {
		panic("bad")
	}
	sort.Slice(images.Images, func(i, j int) bool {
		t1, _ := time.Parse("2006-01-02T15:04:05Z", *images.Images[i].CreationDate)
		t2, _ := time.Parse("2006-01-02T15:04:05Z", *images.Images[j].CreationDate)
		return t1.Unix() < t2.Unix()
	})
	ami := images.Images[len(images.Images)-1].ImageId

	password, err := GenerateRandomString(32)
	if err != nil {
		panic(err)
	}
	UserData := b64.StdEncoding.EncodeToString([]byte(fmt.Sprintf(`#!/bin/bash
	sed 's/PasswordAuthentication no/PasswordAuthentication yes/' -i /etc/ssh/sshd_config
	systemctl restart sshd
	echo "ubuntu:%s" | chpasswd`, password)))
	runInstancesOutput, err := c.Aws.RunInstances(context.Background(), &ec2.RunInstancesInput{
		MinCount:       aws.Int32(1),
		MaxCount:       aws.Int32(1),
		ImageId:        ami,
		InstanceType:   types.InstanceTypeT3Medium,
		UserData:       &UserData,
		SecurityGroups: []string{c.Aws.sg},
	})
	if err != nil {
		panic(err.Error())
	}
	describeInstancesInput := &ec2.DescribeInstancesInput{
		InstanceIds: []string{*runInstancesOutput.Instances[0].InstanceId},
	}
	waiter := ec2.NewInstanceRunningWaiter(c.Aws)
	if err := waiter.Wait(context.Background(), describeInstancesInput, time.Minute*5); err != nil {
		panic(err)
	}
	describeInstancesOutput, err := c.Aws.DescribeInstances(context.Background(), describeInstancesInput)
	if err != nil {
		panic(err)
	}
	fmt.Println(*describeInstancesOutput.Reservations[0].Instances[0].PublicIpAddress)
	return c.spawnResponse(*describeInstancesOutput.Reservations[0].Instances[0].PublicIpAddress, password)
}

func (c *clouds) spawnResponse(ip string, password string) *api.InteractionResponse {
	response := fmt.Sprintf("Your new server located at `%s` with password `%s` is now up!", ip, password)
	return &api.InteractionResponse{
		Type: api.UpdateMessage,
		Data: &api.InteractionResponseData{
			Content: option.NewNullableString(response),
			Components: discord.ComponentsPtr(
				&discord.ActionRowComponent{
					&discord.ButtonComponent{
						Label:    "Destroy",
						CustomID: "destroy",
						Style:    discord.DangerButtonStyle(),
					},
				}),
		}}
}
