import axios from 'axios'

const client = axios.create({
  baseURL: '/api',
  headers: { 'Content-Type': 'application/json' },
  timeout: 30_000,
  withCredentials: true,
})

// Response interceptor – unwrap the envelope, redirect on 401
client.interceptors.response.use(
  (res) => res,
  (err) => {
    if (err.response?.status === 401) {
      const onPublicAuthRoute =
        window.location.pathname.startsWith('/login') ||
        window.location.pathname.startsWith('/register')
      if (!onPublicAuthRoute) {
        window.location.href = '/login'
      }
    }
    const requestId = err.response?.data?.request_id
    const msg = err.response?.data?.error ?? err.message
    const detail = requestId ? `${msg} (Request ID: ${requestId})` : msg
    return Promise.reject(new Error(detail))
  },
)

export default client
