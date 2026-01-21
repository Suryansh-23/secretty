class Secretty < Formula
  desc "PTY wrapper that redacts secrets from terminal output"
  homepage "https://github.com/suryansh-23/secretty"
  url "https://github.com/suryansh-23/secretty/archive/refs/tags/v0.1.0.tar.gz"
  sha256 "REPLACE_WITH_SHA256"
  license "MIT"

  depends_on "go" => :build

  def install
    system "go", "build", "-o", bin/"secretty", "./cmd/secretty"
  end

  test do
    system bin/"secretty", "--help"
  end
end
