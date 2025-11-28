class Lolcathost < Formula
  desc "Dynamic host management tool for macOS and Linux with TUI"
  homepage "https://github.com/lukaszraczylo/lolcathost"
  license "MIT"

  version "0.1.0"

  on_macos do
    on_arm do
      url "https://github.com/lukaszraczylo/lolcathost/releases/download/v#{version}/lolcathost-#{version}-darwin-arm64.tar.gz"
      sha256 "PLACEHOLDER_SHA256_DARWIN_ARM64"
    end

    on_intel do
      url "https://github.com/lukaszraczylo/lolcathost/releases/download/v#{version}/lolcathost-#{version}-darwin-amd64.tar.gz"
      sha256 "PLACEHOLDER_SHA256_DARWIN_AMD64"
    end
  end

  on_linux do
    on_arm do
      url "https://github.com/lukaszraczylo/lolcathost/releases/download/v#{version}/lolcathost-#{version}-linux-arm64.tar.gz"
      sha256 "PLACEHOLDER_SHA256_LINUX_ARM64"
    end

    on_intel do
      url "https://github.com/lukaszraczylo/lolcathost/releases/download/v#{version}/lolcathost-#{version}-linux-amd64.tar.gz"
      sha256 "PLACEHOLDER_SHA256_LINUX_AMD64"
    end
  end

  def install
    bin.install "lolcathost"
  end

  def caveats
    <<~EOS
      lolcathost requires root access for the daemon to modify /etc/hosts.

      After installation:
        1. Run: sudo lolcathost --install
           This will install the LaunchDaemon (macOS) or systemd service (Linux)

        2. Create a config file at ~/.config/lolcathost/config.yaml

        3. Run: lolcathost
           This launches the TUI for managing host entries

      For more information:
        https://github.com/lukaszraczylo/lolcathost
    EOS
  end

  test do
    assert_match version.to_s, shell_output("#{bin}/lolcathost --version")
  end
end
