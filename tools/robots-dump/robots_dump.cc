// robots_dump — prints google/robotstxt's deserialization of a robots.txt
// file as a machine-readable event stream, one record per RobotsParseHandler
// callback:
//
//   START
//   KIND \t line \t base64(key) \t base64(value)
//   END
//
// KIND is USER_AGENT / ALLOW / DISALLOW / SITEMAP / UNKNOWN (robots.cc
// KeyType). key is only populated for UNKNOWN (HandleUnknownAction is the
// only handler that receives it). Values are base64-armored so tabs,
// newlines-in-theory and arbitrary bytes survive the TSV framing.
//
// This is OUR tool, not vendored: it lives outside src-google/ so the
// vendored tree stays byte-identical to upstream. The gluon side
// (src-gluon/google.go) consumes this output to cross-check that both
// parsers produce the same deserialized data.

#include <cstdio>
#include <fstream>
#include <iostream>
#include <sstream>
#include <string>

#include "absl/strings/escaping.h"
#include "absl/strings/string_view.h"
#include "src-google/robots.h"

namespace {

class DumpHandler : public googlebot::RobotsParseHandler {
 public:
  void HandleRobotsStart() override { std::cout << "START\n"; }
  void HandleRobotsEnd() override { std::cout << "END\n"; }

  void HandleUserAgent(int line_num, absl::string_view value) override {
    Emit("USER_AGENT", line_num, "", value);
  }
  void HandleAllow(int line_num, absl::string_view value) override {
    Emit("ALLOW", line_num, "", value);
  }
  void HandleDisallow(int line_num, absl::string_view value) override {
    Emit("DISALLOW", line_num, "", value);
  }
  void HandleSitemap(int line_num, absl::string_view value) override {
    Emit("SITEMAP", line_num, "", value);
  }
  void HandleUnknownAction(int line_num, absl::string_view action,
                           absl::string_view value) override {
    Emit("UNKNOWN", line_num, action, value);
  }

 private:
  static void Emit(const char* kind, int line_num, absl::string_view key,
                   absl::string_view value) {
    std::cout << kind << '\t' << line_num << '\t' << absl::Base64Escape(key)
              << '\t' << absl::Base64Escape(value) << '\n';
  }
};

}  // namespace

int main(int argc, char** argv) {
  if (argc != 2) {
    std::cerr << "usage: robots_dump <robots.txt file>\n";
    return 2;
  }
  std::ifstream f(argv[1], std::ios::binary);
  if (!f.is_open()) {
    std::cerr << "robots_dump: cannot open " << argv[1] << "\n";
    return 2;
  }
  std::stringstream buf;
  buf << f.rdbuf();
  const std::string body = buf.str();

  DumpHandler handler;
  googlebot::ParseRobotsTxt(body, &handler);
  return 0;
}
