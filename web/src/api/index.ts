import request from './request'
import type { LoginRequest, LoginResponse, Network, CreateNetworkRequest, Member, AuthorizeMemberRequest, Peer } from '@/types'

// Auth API
export const authApi = {
  login: (data: LoginRequest) => request.post<any, LoginResponse>('/auth/login', data),
  register: (data: LoginRequest) => request.post('/auth/register', data),
}

// Network API
export const networkApi = {
  list: () => request.get<any, Network[]>('/networks'),
  get: (id: number) => request.get(`/networks/${id}`),
  create: (data: CreateNetworkRequest) => request.post('/networks', data),
  update: (id: number, data: Partial<CreateNetworkRequest>) => request.put(`/networks/${id}`, data),
  delete: (id: number) => request.delete(`/networks/${id}`),
}

// Member API
export const memberApi = {
  list: (networkId: number) => request.get<any, Member[]>(`/networks/${networkId}/members`),
  authorize: (networkId: number, data: AuthorizeMemberRequest) =>
    request.post(`/networks/${networkId}/members`, data),
  update: (networkId: number, nodeAddress: string, data: Partial<AuthorizeMemberRequest>) =>
    request.put(`/networks/${networkId}/members/${nodeAddress}`, data),
  remove: (networkId: number, nodeAddress: string) =>
    request.delete(`/networks/${networkId}/members/${nodeAddress}`),
}

// Peer API
export const peerApi = {
  list: () => request.get<any, Peer[]>('/peers'),
}
