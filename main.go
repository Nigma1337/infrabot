package main

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"os"
	"os/signal"
	"time"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/diamondburned/arikawa/v3/api"
	"github.com/diamondburned/arikawa/v3/api/cmdroute"
	"github.com/diamondburned/arikawa/v3/discord"
	"github.com/diamondburned/arikawa/v3/gateway"
	"github.com/diamondburned/arikawa/v3/state"
	"github.com/diamondburned/arikawa/v3/utils/json/option"
	"github.com/hetznercloud/hcloud-go/hcloud"
	"github.com/spf13/viper"
)

// To run, do `BOT_TOKEN="TOKEN HERE" go run .`

var ROOT_SSH = hcloud.SSHKey{
	ID:        6321437,
	Name:      "mgs",
	PublicKey: "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAINkbG0iV8nP6SRZub/w104jhbZUmAxX6ClHzKJWoX4lI ",
}

var ROOT_SSH_ARRAY = []*hcloud.SSHKey{
	&ROOT_SSH,
}

var commands = []api.CreateCommandData{
	{
		Name:        "ping",
		Description: "ping pong!",
	},
	{
		Name:        "echo",
		Description: "echo back the argument",
		Options: []discord.CommandOption{
			&discord.StringOption{
				OptionName:  "argument",
				Description: "what's echoed back",
				Required:    true,
			},
		},
	},
	{
		Name:        "thonk",
		Description: "biiiig thonk",
	},
	{
		Name:        "locate",
		Description: "locate a server",
		Options: []discord.CommandOption{
			&discord.StringOption{
				OptionName:  "ip",
				Description: "ip to locate",
				Required:    true,
			},
		},
	},
	{
		Name:        "spawn",
		Description: "spawn a server",
	},
}

func BoolPointer(b bool) *bool {
	return &b
}

func main() {
	// https://discord.com/oauth2/authorize?client_id=1093259327353143366&scope=bot&permissions=2147552320
	viper.SetConfigName("config")
	viper.SetConfigType("json")
	viper.AddConfigPath(".")
	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			fmt.Println("Config file not found! Please create config.json")
			os.Exit(1)
		} else {
			panic(err)
		}
	}
	token := viper.GetString("discord.token")
	if token == "" {
		log.Fatalln("No $BOT_TOKEN given.")
	}
	h := newHandler(state.New("Bot " + token))
	h.l = newLocator()
	h.s.AddInteractionHandler(h)
	h.s.AddIntents(gateway.IntentGuilds)
	h.s.AddHandler(func(*gateway.ReadyEvent) {
		me, _ := h.s.Me()
		log.Println("connected to the gateway as", me.Tag())
	})
	h.s.AddHandler(func(ev *gateway.InteractionCreateEvent) {
		switch d := ev.Data.(type) {
		case *discord.ButtonInteraction:
			switch d.ID() {
			case "spawn":
				cloud := ev.InteractionEvent.Message.Components.Find("cloud").(*discord.StringSelectComponent).Placeholder
				region := ev.InteractionEvent.Message.Components.Find("region").(*discord.StringSelectComponent).Placeholder
				ctx := context.Background()
				switch cloud {
				case "hetzner":
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
					res, _, err := h.provider.Hetzner.Server.Create(ctx, opts)
					if err != nil {
						panic(err.Error())
					}
					ip := res.Server.PublicNet.IPv4.IP.String()
					response := fmt.Sprintf("Your new server located at `%s` with password `%s` is now up!", ip, res.RootPassword)
					resp := api.InteractionResponse{
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
					if err := h.s.RespondInteraction(ev.ID, ev.Token, resp); err != nil {
						log.Println("failed to send interaction callback:", err)
					}
				case "aws":
					//instance, err := h.provider.Aws.RunInstances(&ec2.RunInstancesInput{
					//	ImageId: aws.String("a"),
					//})
					//if err != nil {
					//	panic("bad!")
					//}
				}
			}
		case *discord.StringSelectInteraction:
			if d.CustomID == "cloud" {
				log.Println(d.Values[0], d.Values)
				opts, regions := h.getCloudSelection(d.Values[0], "")
				resp := api.InteractionResponse{
					Type: api.UpdateMessage,
					Data: &api.InteractionResponseData{
						Content: option.NewNullableString("Wiener"),
						Components: discord.ComponentsPtr(
							opts,
							regions,
							&discord.ActionRowComponent{
								&discord.ButtonComponent{
									Label:    "Spawn",
									CustomID: "spawn",
									Style:    discord.SuccessButtonStyle(),
								},
							}),
					}}
				if err := h.s.RespondInteraction(ev.ID, ev.Token, resp); err != nil {
					log.Println("failed to send interaction callback:", err)
				}
			} else {
				cloud := ev.InteractionEvent.Message.Components.Find("cloud").(*discord.StringSelectComponent)
				_, regions := h.getCloudSelection(cloud.Placeholder, d.Values[0])
				resp := api.InteractionResponse{
					Type: api.UpdateMessage,
					Data: &api.InteractionResponseData{
						Content: option.NewNullableString("Where do you want the server, buddy?"),
						Components: discord.ComponentsPtr(
							cloud,
							regions,
							&discord.ActionRowComponent{
								&discord.ButtonComponent{
									Label:    "Spawn",
									CustomID: "spawn",
									Style:    discord.SuccessButtonStyle(),
								},
							}),
					}}
				if err := h.s.RespondInteraction(ev.ID, ev.Token, resp); err != nil {
					log.Println("failed to send interaction callback:", err)
				}
			}
		default:
			return
		}
	})
	if err := overwriteCommands(h.s); err != nil {
		log.Fatalln("cannot update commands:", err)
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	if err := h.s.Connect(ctx); err != nil {
		log.Fatalln("cannot connect:", err)
	}
}

func overwriteCommands(s *state.State) error {
	return cmdroute.OverwriteCommands(s, commands)
}

type handler struct {
	*cmdroute.Router
	s        *state.State
	l        *locator
	provider *clouds
}

type clouds struct {
	Hetzner   *hcloud.Client
	Aws       *ec2.Client
	Avaliable []string
}

func newHandler(s *state.State) *handler {
	provider := initClouds()
	h := &handler{s: s, provider: provider}

	h.Router = cmdroute.NewRouter()
	// Automatically defer handles if they're slow.
	h.Use(cmdroute.Deferrable(s, cmdroute.DeferOpts{}))
	h.AddFunc("ping", h.cmdPing)
	h.AddFunc("echo", h.cmdEcho)
	h.AddFunc("thonk", h.cmdThonk)
	h.AddFunc("locate", h.cmdLocate)
	h.AddFunc("spawn", h.cmdSpawn)

	return h
}

func initClouds() *clouds {
	providers := clouds{}
	configuredClouds := viper.Sub("clouds")
	for i, key := range viper.GetStringMap("clouds") {
		str := fmt.Sprintf("%v", key)
		if str == "map[]" {
			continue
		}
		switch i {
		case "hetzner":
			token := configuredClouds.GetString("hetzner.token")
			providers.Hetzner = hcloud.NewClient(hcloud.WithToken(token))
			providers.Avaliable = append(providers.Avaliable, "hetzner")
		case "gcp":
			fmt.Println("gcp")
		case "aws":
			creds := credentials.NewStaticCredentialsProvider(configuredClouds.GetString("aws.AWS_ACCESS_KEY_ID"), configuredClouds.GetString("aws.AWS_SECRET_ACCESS_KEY"), "")
			//region := configuredClouds.GetString("aws.AWS_REGION")
			config, err := config.LoadDefaultConfig(context.Background(), config.WithRegion(""), config.WithCredentialsProvider(creds))
			if err != nil {
				panic(err)
			}
			config.Region = "us-east-1"
			providers.Aws = ec2.NewFromConfig(config)
			_, err = providers.Aws.DescribeInstances(context.Background(), nil)
			if err != nil {
				panic(err)
			}
			providers.Avaliable = append(providers.Avaliable, "aws")
		case "azure":
			fmt.Println("azure")
		}
	}
	return &providers
}

func (h *handler) cmdPing(ctx context.Context, cmd cmdroute.CommandData) *api.InteractionResponseData {
	return &api.InteractionResponseData{
		Content: option.NewNullableString("Pong!"),
	}
}

func (h *handler) cmdEcho(ctx context.Context, data cmdroute.CommandData) *api.InteractionResponseData {
	var options struct {
		Arg string `discord:"argument"`
	}

	if err := data.Options.Unmarshal(&options); err != nil {
		return errorResponse(err)
	}

	return &api.InteractionResponseData{
		Content:         option.NewNullableString(options.Arg),
		AllowedMentions: &api.AllowedMentions{}, // don't mention anyone
	}
}

func (h *handler) cmdThonk(ctx context.Context, data cmdroute.CommandData) *api.InteractionResponseData {
	time.Sleep(time.Duration(3+rand.Intn(5)) * time.Second)
	return &api.InteractionResponseData{
		Content: option.NewNullableString("https://tenor.com/view/thonk-thinking-sun-thonk-sun-thinking-sun-gif-14999983"),
	}
}

func (h *handler) cmdLocate(ctx context.Context, data cmdroute.CommandData) *api.InteractionResponseData {
	var options struct {
		Arg string `discord:"ip"`
	}

	if err := data.Options.Unmarshal(&options); err != nil {
		return errorResponse(err)
	}
	location := h.l.locate(options.Arg)
	return &api.InteractionResponseData{
		Content: option.NewNullableString(location.Region),
	}
}

func (h *handler) cmdSpawn(ctx context.Context, data cmdroute.CommandData) *api.InteractionResponseData {
	opts, regions := h.getCloudSelection("", "")
	return &api.InteractionResponseData{
		Content: option.NewNullableString("Where do you want the server, buddy?"),
		Components: discord.ComponentsPtr(
			opts,
			regions,
			&discord.ActionRowComponent{
				&discord.ButtonComponent{
					Label:    "Spawn",
					CustomID: "spawn",
					Style:    discord.SuccessButtonStyle(),
				},
			},
		),
	}
}

func (h *handler) getCloudSelection(cloud string, region string) (*discord.StringSelectComponent, *discord.StringSelectComponent) {
	if region == "" {
		region = "nowhere"
	}
	return &discord.StringSelectComponent{
			CustomID:    "cloud",
			Options:     h.provider.getCloudOpts(cloud),
			Placeholder: cloud,
		},
		&discord.StringSelectComponent{
			CustomID:    "region",
			Options:     h.provider.getCloudRegions(cloud, region),
			Placeholder: region,
		}
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
		//565feec9-3d43-413e-9760-c651546613f2
		images, err := c.Aws.DescribeImages(ctx, &ec2.DescribeImagesInput{
			Filters: []types.Filter{
				{
					Name:   aws.String("owner-id"),
					Values: []string{"099720109477"}, // Cannonical ownership id, see https://ubuntu.com/server/docs/cloud-images/amazon-ec2
				},
				{
					Name:   aws.String("creation-date"),
					Values: []string{"099720109477"},
				},
			},
		},
		)
		if err != nil {
			panic("bad")
		}
		for _, image := range images.Images {
			log.Printf("%s", *image.Name)
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

func errorResponse(err error) *api.InteractionResponseData {
	return &api.InteractionResponseData{
		Content:         option.NewNullableString("**Error:** " + err.Error()),
		Flags:           discord.EphemeralMessage,
		AllowedMentions: &api.AllowedMentions{ /* none */ },
	}
}
