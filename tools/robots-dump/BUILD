# robots_dump: event-stream dumper over the vendored google parser. Kept
# outside src-google/ so the vendored tree stays pristine (see
# src-google/VENDOR.md).
load("@rules_cc//cc:defs.bzl", "cc_binary")

package(default_visibility = ["//visibility:public"])

cc_binary(
    name = "robots_dump",
    srcs = ["robots_dump.cc"],
    deps = [
        "//src-google:robots",
        "@abseil-cpp//absl/strings",
    ],
)
