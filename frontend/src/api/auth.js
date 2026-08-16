import { http } from './http.js'

export const authApi = {
  login: (payload) => http.post('/login', payload),
  register: (payload) => http.post('/register', payload),
}
