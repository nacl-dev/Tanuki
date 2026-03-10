import axios from 'axios'

const client = axios.create({
  baseURL: '/api',
  headers: { 'Content-Type': 'application/json' },
  timeout: 30_000,
})

// Request interceptor – attach Bearer token when available
client.interceptors.request.use((config) => {
  const token = localStorage.getItem('tanuki_token')
  if (token) {
    config.headers.Authorization = `Bearer ${token}`
  }
  return config
})

// Response interceptor – unwrap the envelope, redirect on 401
client.interceptors.response.use(
  (res) => res,
  (err) => {
    if (err.response?.status === 401) {
      localStorage.removeItem('tanuki_token')
      window.location.href = '/login'
    }
    const msg = err.response?.data?.error ?? err.message
    return Promise.reject(new Error(msg))
  },
)

export default client
