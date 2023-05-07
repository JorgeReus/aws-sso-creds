# aws-sso-creds

<div id="top"></div>
<!-- PROJECT LOGO -->
<br />
<div align="center">

  <img src="./.md/aws-sso-creds.gif" />
  <br>
  <img src="./.md/previewer.gif" />
  <br>
  <img src="./.md/gopher.png" alt="Logo" width="80" height="80">

  <h3 align="center">AWS SSO Creds</h3>
</div>

<!-- TABLE OF CONTENTS -->
<details>
  <summary>Table of Contents</summary>
  <ol>
    <li>
      <a href="#about">About The Project</a>
      <ul>
        <li><a href="#built-with">Built With</a></li>
      </ul>
    </li>
    <li>
      <a href="#installation">Instalation</a>
      <ul>
        <li><a href="#static-releases">Static</a></li>
        <li><a href="#from-source">From source</a></li>
      </ul>
    </li>
    <li><a href="#usage">Usage</a></li>
    <li><a href="#configuration">Configuration</a></li>
    <li><a href="#contributing">Contributing</a></li>
    <li><a href="#license">License</a></li>
    <li><a href="#contact">Contact</a></li>
    <li><a href="#acknowledgments">Acknowledgments</a></li>
  </ol>
</details>

## About

Opinionated CLI app for AWS SSO made in Golang!

AWS SSO Creds is an AWS SSO creds manager for the shell.

Use it to **easily** manage entries in `~/.aws/config` & `~/.aws/credentials` files, so you can focus on your AWS workflows, without the hazzle of manually managing your credentials.

### Built With

- [Bubbletea](https://github.com/charmbracelet/bubbletea)
- [Go-fuzzyfinder](https://github.com/ktr0731/go-fuzzyfinder)

<!-- GETTING STARTED -->

## Installation

### Static Releases

Download the binary based on your OS in [The releases section](https://github.com/JorgeReus/aws-sso-creds/releases)

### From source

#### Prerequisites

- Go 1.17+

Run `go install github.com/JorgeReus/aws-sso-creds@latest`

<!-- USAGE EXAMPLES -->

## Usage

```
Opinionated CLI app for AWS SSO made in Golang!
AWS SSO Creds is an AWS SSO creds manager for the shell.
Use it to easily manage entries in ~/.aws/config & ~/.aws/credentials files, so you can focus on your AWS workflows, without the hazzle of manually managing your credentials.

Usage:
  aws-sso-creds [flags] [organization]
  aws-sso-creds [command]

Available Commands:
  completion  Generate the autocompletion script for the specified shell
  help        Help about any command
  open        Opens the AWS web console based on your AWS_PROFILE environment variable
  select      Select your role/credentials in a fuzzy-finder previewer

Flags:
  -c, --config string   Directory of the .toml config (default "/home/reus/.config/aws-sso-creds.toml")
  -f, --forceAuth       Force Authentication with AWS SSO
  -h, --help            help for aws-sso-creds
  -b, --noBrowser       Do not open in the browser automatically
  -p, --populateRoles   Populate AWS SSO roles in ~/.aws/config
  -t, --temp            Create temporary credentials in ~/.aws/credentials

Use "aws-sso-creds [command] --help" for more information about a command.
```

## Configuration

This tool required a `toml` config file located in ~/.config/aws-sso-creds.toml by default, take a look at the following example:

```
error_color = "#fa0718"
information_color = "#05fa5f"
warning_color = "#f29830"
focus_color = "#4287f5"
spinner_color = "#42f551"

[organizations.org1]
url = "https://org1.awsapps.com/start"
prefix = "org1"
region = "us-east-1"

[organizations.org2]
url = "https://org2.awsapps.com/start"
prefix = "org2"
region = "us-west-2"
```

Each organization entry must have:

- url: The awsapps URL to interact with the AWS SSO/Identity Center Org
- prefix: A prefix to identify profiles in the aws local config files
- region: The region of the AWS SSO/Identity Center Org

The color entries must be defined in `hex` notation

<!-- CONTRIBUTING -->

## Contributing

Contributions are what make the open source community such an amazing place to learn, inspire, and create. Any contributions you make are **greatly appreciated**.

If you have a suggestion that would make this better, please fork the repo and create a pull request. You can also simply open an issue with the tag "enhancement".
Don't forget to give the project a star! Thanks again!

1. Fork the Project
2. Create your Feature Branch (`git checkout -b feature/AmazingFeature`)
3. Commit your Changes (`git commit -m 'Add some AmazingFeature'`)
4. Push to the Branch (`git push origin feature/AmazingFeature`)
5. Open a Pull Request

<!-- LICENSE -->

## License

Distributed under the MIT License. See `LICENSE` for more information.

<!-- CONTACT -->

## Contact

Jorge Reus - [LinkedIn](www.linkedin.com/in/JorgeGReus)

<!-- ACKNOWLEDGMENTS -->

## Acknowledgments

- [TLDR Legal](https://tldrlegal.com/)
- [Build your own Gopher](https://quasilyte.dev/)
- [Bubbles components](https://github.com/charmbracelet/bubbles)
- [Python implementation of this tool](https://github.com/benkehoe/aws-sso-util)
- [Best README Template](https://github.com/othneildrew/Best-README-Template)
