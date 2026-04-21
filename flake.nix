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

          # buildGoModule's wrapped Go toolchain overrides GOOS/GOARCH at the
          # toolchain level regardless of env, so cross-compilation to wasip1 needs
          # a mkDerivation that calls Go directly.  We reuse the vendor directory
          # that buildGoModule already fetched (common.goModules passthru).
          wasm =
            let
              goModules = (pkgs.buildGoModule common).goModules;
            in
            pkgs.stdenv.mkDerivation {
              pname = "sqlc-gen-template-wasm";
              inherit version;
              src = pkgs.lib.cleanSource ./.;
              nativeBuildInputs = [ pkgs.go ];
              buildPhase = ''
                export HOME=$TMPDIR
                cp -r ${goModules} vendor
                chmod -R u+w vendor
                CGO_ENABLED=0 GOOS=wasip1 GOARCH=wasm \
                  go build -mod=vendor -o sqlc-gen-template.wasm \
                  ./cmd/sqlc-gen-template
              '';
              installPhase = ''
                mkdir -p "$out/bin"
                mv sqlc-gen-template.wasm "$out/bin/"
              '';
              doCheck = false;
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
