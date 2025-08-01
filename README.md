# Infrabot

Discord bot mainly made for spawning infrastructure for CTFs

Currently supports Hetzner and AWS

## Setup

- Copy `config.json.example` to `config.json` and fill in your credentials.
- Run with:  
  `go run .`

### Extra setup

If you want the locator to also create Azure, you'll need to download the json with azure ips from `https://www.microsoft.com/en-us/download/details.aspx?id=56519` and put it in the file "azure_ips.json"

## TODO
- Finish GCP, DO and Azure
- Instance type selection