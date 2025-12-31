variable "GITHUB_SHA" { default = "latest" }
variable "VERSION" { default = "dev" }

group "default" {
  targets = ["bd-drain"]
}

target "bd-drain" {
  context    = "."
  dockerfile = "Dockerfile"
  args = {
    VERSION = "${VERSION}"
  }
  output = [
    "type=image,oci-mediatypes=true,compression=zstd,compression-level=22,force-compression=true",
  ]
  tags = [
    "ghcr.io/npratt/bd-drain:${GITHUB_SHA}",
  ]
}
