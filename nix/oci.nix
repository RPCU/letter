{
  pkgs ? import <nixpkgs> { },
  dockerVersion ? "0.0.0",
  commit ? "unknown",
  doCheck ? true,
  imageName ? "zot.rpcu.io/public/letter",
}:
let
  binaries = pkgs.callPackage ./binaries.nix {
    version = dockerVersion;
    inherit commit doCheck;
  };
  # Claude Code (unfree) writes the AI notes.
  unfree = import pkgs.path {
    inherit (pkgs) system;
    config.allowUnfreePredicate = p: pkgs.lib.getName p == "claude-code";
  };
in
pkgs.dockerTools.streamLayeredImage {
  name = imageName;
  tag = dockerVersion;
  contents = [
    binaries
    unfree.claude-code
    # A shell to sign Claude Code in: `claude auth login`.
    pkgs.bashInteractive
    pkgs.coreutils
    pkgs.cacert
    (pkgs.dockerTools.fakeNss.override {
      extraPasswdLines = [ "letter:x:1001:0:Letter:/home/letter:/bin/bash" ];
    })
  ];
  fakeRootCommands = ''
    ln -s bin/letter letter
    mkdir -p home/letter tmp
    chown 1001:0 home/letter
    chmod 0770 home/letter
    chmod 1777 tmp
  '';
  config = {
    User = "1001:0";
    Entrypoint = [ "/letter" ];
    # OpenChoreo workloads cannot set command/args, so the image's default
    # command must start the server.
    Cmd = [ "serve" ];
    WorkingDir = "/home/letter";
    Env = [
      "HOME=/home/letter"
      # The AI notes' Claude login, apart from any other settings.
      "CLAUDE_CONFIG_DIR=/home/letter/.claude"
      "DISABLE_AUTOUPDATER=1"
      "PATH=/bin"
      "SSL_CERT_FILE=/etc/ssl/certs/ca-bundle.crt"
    ];
    ExposedPorts."8080/tcp" = { };
  };
}
