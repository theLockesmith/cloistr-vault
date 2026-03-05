import axios, { AxiosInstance, AxiosError } from 'axios';
import AsyncStorage from '@react-native-async-storage/async-storage';

const API_BASE_URL = __DEV__
  ? 'http://localhost:8080/api/v1'
  : 'https://vault.cloistr.xyz/api/v1';

class ApiService {
  private client: AxiosInstance;

  constructor() {
    this.client = axios.create({
      baseURL: API_BASE_URL,
      timeout: 30000,
      headers: {
        'Content-Type': 'application/json',
      },
    });

    this.client.interceptors.request.use(async (config) => {
      const token = await AsyncStorage.getItem('vault_token');
      if (token) {
        config.headers.Authorization = `Bearer ${token}`;
      }
      return config;
    });

    this.client.interceptors.response.use(
      (response) => response,
      (error: AxiosError) => {
        if (error.response?.status === 401) {
          AsyncStorage.removeItem('vault_token');
          AsyncStorage.removeItem('vault_user');
        }
        return Promise.reject(error);
      }
    );
  }

  // Auth endpoints
  async login(email: string, password: string) {
    const response = await this.client.post('/auth/login', {
      method: 'email',
      email,
      password,
    });
    return response.data;
  }

  // WebAuthn endpoints
  async webauthnLoginBegin(email: string) {
    const response = await this.client.post('/auth/webauthn/login/begin', { email });
    return response.data;
  }

  async webauthnLoginBeginDiscoverable() {
    const response = await this.client.post('/auth/webauthn/login/begin/discoverable');
    return response.data;
  }

  async webauthnLoginFinish(credential: any) {
    const response = await this.client.post('/auth/webauthn/login/finish', credential);
    return response.data;
  }

  async webauthnRegisterBegin() {
    const response = await this.client.post('/user/webauthn/register/begin');
    return response.data;
  }

  async webauthnRegisterFinish(credential: any) {
    const response = await this.client.post('/user/webauthn/register/finish', credential);
    return response.data;
  }

  async getWebauthnCredentials() {
    const response = await this.client.get('/user/webauthn/credentials');
    return response.data;
  }

  async renameWebauthnCredential(id: string, name: string) {
    const response = await this.client.put(`/user/webauthn/credentials/${id}`, { name });
    return response.data;
  }

  async deleteWebauthnCredential(id: string) {
    const response = await this.client.delete(`/user/webauthn/credentials/${id}`);
    return response.data;
  }

  // Health check
  async healthCheck() {
    const response = await this.client.get('/health');
    return response.data;
  }
}

export const api = new ApiService();
export default api;
