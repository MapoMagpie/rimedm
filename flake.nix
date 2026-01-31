{
  description = "rimedm";

  inputs = {
    nixpkgs.url = "github:nixos/nixpkgs?ref=nixos-unstable";
    systems.url = "github:nix-systems/default";
  };

  outputs =
    {
      self,
      nixpkgs,
      systems,
      ...
    }:
    let
      forEachSystem = nixpkgs.lib.genAttrs (import systems);
      pkgsFor = forEachSystem (system: import nixpkgs { inherit system; });
    in
    {
      formatter = forEachSystem (system: pkgsFor.${system}.alejandra);

      devShells = forEachSystem (system: {
        default = pkgsFor.${system}.mkShell {
          packages = with pkgsFor.${system}; [
            go
            gopls
            golangci-lint
            golangci-lint-langserver
          ];
          shellHook = "exec zsh";
        };
      });

      packages = forEachSystem (system: {
        default = pkgsFor.${system}.buildGoModule {
          pname = "rimedm";
          version = "1.1.5";
          src = ./.;
          vendorHash = "sha256-x8d0EAKcPJa0VfhUdlFN/GoEK2ZP/DAJQToBkv2/gao=";
        };
      });

      apps = forEachSystem (system: {
        default = {
          type = "app";
          program = "${self.packages.${system}.default}/bin/rimedm";
        };
      });
    };
}
