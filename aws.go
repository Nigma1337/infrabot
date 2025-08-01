package main

import (
	"context"
	"encoding/base64"
	"fmt"
	"sort"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/diamondburned/arikawa/v3/api"
	"github.com/diamondburned/arikawa/v3/api/cmdroute"
)

type awooga struct {
	*ec2.Client
	sg string
}

func (c *clouds) spawnResponseAWS(ctx context.Context, data cmdroute.ComponentData) *api.InteractionResponse {
	images, err := c.Aws.DescribeImages(ctx, &ec2.DescribeImagesInput{
		Filters: []types.Filter{
			{
				Name:   aws.String("owner-id"),
				Values: []string{"099720109477"},
			},
			{
				Name:   aws.String("creation-date"),
				Values: []string{"2025*"},
			},
			{
				Name:   aws.String("name"),
				Values: []string{"ubuntu/images/hvm-ssd/ubuntu-jammy-24.04-amd64-server-*"},
			},
			{
				Name:   aws.String("architecture"),
				Values: []string{"x86_64"},
			},
		},
	})
	if err != nil {
		panic("bad")
	}
	sort.Slice(images.Images, func(i, j int) bool {
		t1, _ := time.Parse("2006-01-02T15:04:05Z", *images.Images[i].CreationDate)
		t2, _ := time.Parse("2006-01-02T15:04:05Z", *images.Images[j].CreationDate)
		return t1.Unix() < t2.Unix()
	})
	//ami := images.Images[len(images.Images)-1].ImageId

	password, err := GenerateRandomString(32)
	if err != nil {
		panic(err)
	}
	UserData := base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf(`#!/bin/bash
	sed 's/PasswordAuthentication no/PasswordAuthentication yes/' -i /etc/ssh/sshd_config
	rm /etc/ssh/sshd_config.d/60-cloudimg-settings.conf
	systemctl restart ssh
	echo "ubuntu:%s" | chpasswd`, password)))
	runInstancesOutput, err := c.Aws.RunInstances(context.Background(), &ec2.RunInstancesInput{
		MinCount: aws.Int32(1),
		MaxCount: aws.Int32(1),
		// Fetch latest ubuntu, source https://documentation.ubuntu.com/aws/aws-how-to/instances/find-ubuntu-images/
		ImageId:        aws.String("resolve:ssm:/aws/service/canonical/ubuntu/server/24.04/stable/current/amd64/hvm/ebs-gp3/ami-id"),
		InstanceType:   types.InstanceTypeT3Medium,
		UserData:       &UserData,
		SecurityGroups: []string{c.Aws.sg},
		TagSpecifications: []types.TagSpecification{
			{
				ResourceType: types.ResourceTypeInstance,
				Tags: []types.Tag{
					{
						Key:   aws.String("channel"),
						Value: aws.String(data.Event.ChannelID.String()),
					},
					{
						Key:   aws.String("guild"),
						Value: aws.String(data.Event.GuildID.String()),
					},
				},
			},
		},
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
	return c.spawnResponse(*describeInstancesOutput.Reservations[0].Instances[0].PublicIpAddress, password, "ubuntu", "", "aws")
}

func (c *clouds) deleteAWS(ctx context.Context, data cmdroute.ComponentData) *api.InteractionResponse {
	describeInput := &ec2.DescribeInstancesInput{
		Filters: []types.Filter{
			{
				Name:   aws.String("tag:channel"),
				Values: []string{data.Event.ChannelID.String()},
			},
			{
				Name:   aws.String("tag:guild"),
				Values: []string{data.Event.GuildID.String()},
			},
			{
				Name:   aws.String("instance-state-name"),
				Values: []string{"running"},
			},
		},
	}
	instances, err := c.Aws.DescribeInstances(ctx, describeInput)
	if err != nil {
		panic(err)
	}
	if len(instances.Reservations) == 0 || len(instances.Reservations[0].Instances) == 0 {
		fmt.Println("No matching AWS instance found.")
		return c.deleteResponse()
	}
	instanceId := *instances.Reservations[0].Instances[0].InstanceId
	_, err = c.Aws.TerminateInstances(ctx, &ec2.TerminateInstancesInput{
		InstanceIds: []string{instanceId},
	})
	if err != nil {
		panic(err)
	}
	return c.deleteResponse()
}
