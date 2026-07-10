provider "proxmox" {
  endpoint = "https://192.168.1.200:8006/"
  insecure = true
  # api_token is read from PROXMOX_VE_API_TOKEN env var
}
