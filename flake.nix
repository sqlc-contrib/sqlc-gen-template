{
  description = "sqlc-gen-template - sqlc codegen plugin that renders user-supplied Go templates";

  nixConfig = {
    extra-substituters = [ "https://nixos-contrib.cachix.org" ];
    extra-trusted-public-keys = [ "nixos-contrib.cachix.org-1:WRyXCyMQGjYEEC87RdlW4ZbK/R3Ia4/WGYEoIP9X/o0=" ];
  };

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixpkgs-unstable";
    flake-utils.url = "github:numtide/flake-utils";
  };

  outputs =
    { nixpkgs, flake-utils, ... }:
    flake-utils.lib.eachDefaultSystem (
      system:
      let
        pkgs = nixpkgs.legacyPackages.${system};
        version = (pkgs.lib.importJSON ./.github/config/release-please-manifest.json).".";
      in
      {
        packages.default = pkgs.buildGoModule {
          pname = "sqlc-gen-template";
          inherit version;
          src = pkgs.lib.cleanSource ./.;
          subPackages = [ "cmd/sqlc-gen-template" ];
          vendorHash = "sha256-z0mS9GZUsE6xlhDGFbxbNp5wfhop7Wv7FxwzO0kB5PM=";
          meta = with pkgs.lib; {
            description = "sqlc plugin that renders code from Go templates";
            license = licenses.mit;
            mainProgram = "sqlc-gen-template";
          };
        };

        devShells.default = pkgs.mkShell {
          name = "sqlc-gen-template";
          packages = [
            pkgs.go
          ];
        };
      }
    );
}
