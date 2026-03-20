{
  description = "aws-sso-creds";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
  };

  outputs =
    { self, nixpkgs }:
    let
      systems = [
        "x86_64-linux"
        "aarch64-linux"
        "x86_64-darwin"
        "aarch64-darwin"
      ];
      forAllSystems =
        f:
        nixpkgs.lib.genAttrs systems (
          system:
          f {
            pkgs = import nixpkgs { inherit system; };
            inherit system;
          }
        );
      releaseVersion =
        let
          lines = builtins.filter (line: line != "" && !(nixpkgs.lib.strings.hasPrefix "<!--" line)) (
            nixpkgs.lib.strings.splitString "\n" (builtins.readFile ./cmd/aws-sso-creds/release_version.txt)
          );
        in
        if lines == [ ] then "dev" else builtins.head lines;
    in
    {
      packages = forAllSystems (
        { pkgs, system }:
        let
          version = releaseVersion;
        in
        {

          default = pkgs.buildGoModule {
            pname = "aws-sso-creds";
            inherit version;
            src = ./.;
            subPackages = [ "." ];
            vendorHash = "sha256-mVQecwuYO5/XTSREf/nJ+fV6cG8ZoLmd7kMd7/sJXCY=";
            ldflags = [
              "-s"
              "-w"
              "-X github.com/JorgeReus/aws-sso-creds/cmd/aws-sso-creds.version=${version}"
            ];
            meta = {
              description = "Opinionated CLI app for AWS SSO";
              homepage = "https://github.com/JorgeReus/aws-sso-creds";
              mainProgram = "aws-sso-creds";
            };
          };
          aws-sso-creds = self.packages.${system}.default;
        }
      );

      devShells = forAllSystems (
        { pkgs, ... }:
        {
          default = pkgs.mkShell {
            packages = with pkgs; [
              go_1_26
            ];
          };
        }
      );
    };
}
