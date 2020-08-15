{ nixpkgs ? import <nixpkgs> { } }:

let
  pkgs = [ nixpkgs.go nixpkgs.ripgrep ];

in
  nixpkgs.stdenv.mkDerivation {
    name = "env";
    buildInputs = pkgs;
  }
