// API Response types
export interface ApiResponse<T = any> {
  data?: T
  error?: string
  message?: string
}

// Auth types
export interface LoginRequest {
  username: string
  password: string
}

export interface LoginResponse {
  token: string
  expires_at: string
}

export interface User {
  id: number
  username: string
  role: string
}

// Network types
export interface Network {
  id: number
  name: string
  description: string
  ip_range: string
  ip6_range?: string
  mtu: number
  multicast: boolean
  member_count: number
  online_count: number
  created_at: string
}

export interface CreateNetworkRequest {
  name: string
  description?: string
  ip_range: string
  ip6_range?: string
  mtu?: number
  multicast?: boolean
}

// Member types
export interface Member {
  network_id: number
  node_address: string
  authorized: boolean
  ip_address: string
  name: string
  online: boolean
  platform: string
  last_seen: string
  created_at: string
}

export interface AuthorizeMemberRequest {
  node_address: string
  authorized: boolean
  ip_address?: string
  name?: string
}

// Peer types
export interface Peer {
  address: string
  platform: string
  online: boolean
  last_seen: string
}
