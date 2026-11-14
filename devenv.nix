{
  pkgs,
  ...
}:
{
  packages = with pkgs; [
    go
    gofumpt
    golines
    air
  ];

  env.LETTER_NAME = "Chihiro";
  env.LETTER_FROM = "Victor";
  env.LETTER_TIMEZONE = "Europe/Paris";

  enterShell = ''
    echo ""
    echo "Letter development environment loaded"
    echo ""
    echo "Run with:"
    echo "  go run . serve    # http://127.0.0.1:8080"
    echo ""
  '';
}
