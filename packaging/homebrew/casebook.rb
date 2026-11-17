# Homebrew formula template for the casebook CLI (which also serves MCP).
#
# scripts/render-homebrew-formula.sh fills the @...@ placeholders from a
# release's SHA256SUMS.txt; .github/workflows/homebrew.yml commits the result
# to Formula/casebook.rb in the jameskbb/homebrew-tap repository when a
# release is published, so `brew install jameskbb/tap/casebook` works.
# docs/building-releases.md describes the setup. Edit this template, never
# the rendered copy in the tap.
class Casebook < Formula
  desc "Local evidence memory for AI investigations, shared across AI clients"
  homepage "https://github.com/jameskbb/casebook"
  version "@VERSION@"
  license "@LICENSE@"
  on_macos do
    on_arm do
      url "https://github.com/jameskbb/casebook/releases/download/@TAG@/casebook-cli-@TAG@-darwin-arm64.tar.gz"
      sha256 "@SHA256_DARWIN_ARM64@"
    end
    on_intel do
      url "https://github.com/jameskbb/casebook/releases/download/@TAG@/casebook-cli-@TAG@-darwin-amd64.tar.gz"
      sha256 "@SHA256_DARWIN_AMD64@"
    end
  end

  on_linux do
    on_arm do
      url "https://github.com/jameskbb/casebook/releases/download/@TAG@/casebook-cli-@TAG@-linux-arm64.tar.gz"
      sha256 "@SHA256_LINUX_ARM64@"
    end
    on_intel do
      url "https://github.com/jameskbb/casebook/releases/download/@TAG@/casebook-cli-@TAG@-linux-amd64.tar.gz"
      sha256 "@SHA256_LINUX_AMD64@"
    end
  end

  def install
    bin.install "casebook"
    prefix.install "LICENSE" if File.exist?("LICENSE")
    prefix.install "THIRD_PARTY_NOTICES.txt" if File.exist?("THIRD_PARTY_NOTICES.txt")
  end

  def caveats
    <<~EOS
      Connect an AI client to Casebook:
        casebook setup claude
      or `casebook setup codex`, or `casebook setup mcp` for any other MCP client.
      `casebook doctor` checks the install. Your data stays on this machine.
    EOS
  end

  test do
    assert_match "casebook v#{version}", shell_output("#{bin}/casebook --version")
  end
end
