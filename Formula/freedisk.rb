# frozen_string_literal: true

# Template for the personal tap CiprianSpiridon/homebrew-freedisk.
# Copy this file to that repo as Formula/freedisk.rb.
# Until a tagged release exists:
#   brew install --HEAD CiprianSpiridon/freedisk/freedisk
# After v0.1.0: uncomment url/sha256 (see comments below) and bump in the tap.

# macOS disk-audit CLI (scan never deletes).
class Freedisk < Formula
  desc "macOS disk-audit CLI; scan never deletes"
  homepage "https://github.com/CiprianSpiridon/free-disk-space"
  url "https://github.com/CiprianSpiridon/free-disk-space/archive/refs/tags/v0.1.0.tar.gz"
  sha256 "f20b6eda7fe76622418bbc9d69d24636b79c5e1e25366f68428078ab2f195c1e"
  license "MIT"
  head "https://github.com/CiprianSpiridon/free-disk-space.git", branch: "main"

  depends_on "go" => :build
  depends_on :macos

  def install
    ldflags = "-s -w -X github.com/CiprianSpiridon/free-disk-space/internal/version.Version=#{version}"
    system "go", "build", *std_go_args(ldflags: ldflags), "./cmd/freedisk"
  end

  def caveats
    <<~EOS
      Scan never deletes. Only `freedisk delete <id>` removes files, and only
      for the finding ids you name. There is no `delete --all`. Non-TTY
      delete requires `--yes`.
    EOS
  end

  test do
    assert_match version.to_s, shell_output("#{bin}/freedisk version")
    help = shell_output("#{bin}/freedisk help")
    assert_match "never deletes", help
    assert_match "scan", help
  end
end
