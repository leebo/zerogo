import axios, { AxiosInstance, AxiosError, InternalAxiosRequestConfig } from 'axios'
import { cyberMessage } from '@/components/ui/CyberMessage'

// Create axios instance
const request: AxiosInstance = axios.create({
  baseURL: '/api/v1',
  timeout: 10000,
  headers: {
    'Content-Type': 'application/json',
  },
})

// Request interceptor
request.interceptors.request.use(
  (config: InternalAxiosRequestConfig) => {
    const token = localStorage.getItem('token')
    if (token && config.headers) {
      config.headers.Authorization = `Bearer ${token}`
    }
    return config
  },
  (error: AxiosError) => {
    return Promise.reject(error)
  }
)

// Response interceptor
request.interceptors.response.use(
  (response) => {
    return response.data
  },
  (error: AxiosError) => {
    if (error.response) {
      const status = error.response.status
      switch (status) {
        case 401:
          cyberMessage.error('Unauthorized, please login')
          localStorage.removeItem('token')
          window.location.href = '/login'
          break
        case 403:
          cyberMessage.error('Forbidden')
          break
        case 404:
          cyberMessage.error('Resource not found')
          break
        case 500:
          cyberMessage.error('Server error')
          break
        default:
          cyberMessage.error((error.response.data as any)?.error || 'Request failed')
      }
    } else {
      cyberMessage.error('Network error')
    }
    return Promise.reject(error)
  }
)

export default request
