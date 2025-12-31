variable "GITHUB_SHA" { default = "latest" }
variable "VERSION" { default = "dev" }

group "default" {
  targets = ["atari"]
}

target "atari" {
  context    = "."
  dockerfile = "Dockerfile"
  args = {
    VERSION = "${VERSION}"
  }
  output = [
    "type=image,oci-mediatypes=true,compression=zstd,compression-level=22,force-compression=true",
  ]
  tags = [
    "ghcr.io/npratt/atari:${GITHUB_SHA}",
  ]
}
