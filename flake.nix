{
  description = "sqlc-gen-template - sqlc codegen plugin that renders user-supplied Go templates";

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

        common = {
          pname = "sqlc-gen-template";
          inherit version;
          src = pkgs.lib.cleanSource ./.;
          subPackages = [ "cmd/sqlc-gen-template" ];
          vendorHash = "sha256-Xc+AVFZQfYz1mf8+zpIgeTppB6p22x0+20JiHnv2qgE=";
        };
      in
      {
        packages = {
          default = pkgs.buildGoModule (common // {
            meta = with pkgs.lib; {
              description = "sqlc plugin that renders code from Go templates";
              license = licenses.mit;
              mainProgram = "sqlc-gen-template";
            };
          });

          wasm = pkgs.buildGoModule (common // {
            pname = "sqlc-gen-template-wasm";
            env = {
              GOOS = "wasip1";
              GOARCH = "wasm";
            };
            # buildGoModule's default install puts the binary in $out/bin.
            # The wasip1/wasm build emits a .wasm extension automatically.
            postInstall = ''
              mv $out/bin/sqlc-gen-template $out/bin/sqlc-gen-template.wasm || true
              test -f $out/bin/sqlc-gen-template.wasm
            '';
            doCheck = false;
          });
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
