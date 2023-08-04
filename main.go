package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
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

func newHandler(s *state.State) *handler {
	provider := initClouds()
	h := &handler{s: s, provider: provider}

	h.Router = cmdroute.NewRouter()
	// Automatically defer handles if they're slow.
	h.Use(cmdroute.Deferrable(s, cmdroute.DeferOpts{}))
	h.AddFunc("locate", h.cmdLocate)
	h.AddFunc("spawn", h.cmdSpawn)
	h.AddComponentFunc("spawn_hetzner", h.provider.spawnResponseHetzner)
	h.AddComponentFunc("spawnn_aws", h.provider.spawnResponseAWS)
	h.AddComponentFunc("cloud", h.handleCloudSelect)
	h.AddComponentFunc("region", h.handleRegionSelect)
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
			region := configuredClouds.GetString("aws.AWS_REGION")
			config, err := config.LoadDefaultConfig(context.Background(), config.WithRegion(region), config.WithCredentialsProvider(creds))
			if err != nil {
				panic(err)
			}
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

func (h *handler) handleCloudSelect(ctx context.Context, data cmdroute.ComponentData) *api.InteractionResponse {
	value := data.Event.Data.(*discord.StringSelectInteraction).Values[0]
	log.Println(value)
	opts, regions := h.getCloudSelection(value, "")
	resp := api.InteractionResponse{
		Type: api.UpdateMessage,
		Data: &api.InteractionResponseData{
			Content: option.NewNullableString("Where do you want the server, buddy?"),
			Components: discord.ComponentsPtr(
				opts,
				regions,
				&discord.ActionRowComponent{
					&discord.ButtonComponent{
						Label:    "Spawn",
						CustomID: discord.ComponentID("spawn_" + value),
						Style:    discord.SuccessButtonStyle(),
					},
				}),
		}}
	return &resp
}

func (h *handler) handleRegionSelect(ctx context.Context, data cmdroute.ComponentData) *api.InteractionResponse {
	value := data.Event.Data.(*discord.StringSelectInteraction).Values[0]
	cloud := data.Event.Message.Components.Find("cloud").(*discord.StringSelectComponent)
	_, regions := h.getCloudSelection(cloud.Placeholder, value)
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
						CustomID: discord.ComponentID("spawn_" + cloud.Placeholder),
						Style:    discord.SuccessButtonStyle(),
					},
				}),
		}}
	return &resp
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
					CustomID: discord.ComponentID("aws_hetzner"), // hetzner is default cloud
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

func errorResponse(err error) *api.InteractionResponseData {
	return &api.InteractionResponseData{
		Content:         option.NewNullableString("**Error:** " + err.Error()),
		Flags:           discord.EphemeralMessage,
		AllowedMentions: &api.AllowedMentions{ /* none */ },
	}
}
