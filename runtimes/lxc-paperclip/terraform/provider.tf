provider "proxmox" {
  endpoint = "https://192.168.1.200:8006/"
  insecure = true
  # api_token is read from PROXMOX_VE_API_TOKEN env var

  # There is deliberately no `ssh {}` block here: the PVE-side raw configuration
  # (lxc.cap.drop, lxc.cgroup2.io.max) is written by create.sh's phase_raw_config
  # over root SSH with the operator's provisioning key, because an API token
  # cannot set raw keys — the same split the operator runtime uses.
}
