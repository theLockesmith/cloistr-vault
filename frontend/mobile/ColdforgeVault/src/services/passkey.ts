import { Passkey, PasskeyCreateResult, PasskeyGetResult } from 'react-native-passkey';
import { Platform } from 'react-native';
import api from './api';

// Base64URL encoding/decoding utilities
function base64UrlEncode(buffer: ArrayBuffer): string {
  const bytes = new Uint8Array(buffer);
  let binary = '';
  for (let i = 0; i < bytes.byteLength; i++) {
    binary += String.fromCharCode(bytes[i]);
  }
  return btoa(binary)
    .replace(/\+/g, '-')
    .replace(/\//g, '_')
    .replace(/=/g, '');
}

function base64UrlDecode(str: string): ArrayBuffer {
  const padding = '='.repeat((4 - (str.length % 4)) % 4);
  const base64 = (str + padding).replace(/-/g, '+').replace(/_/g, '/');
  const binary = atob(base64);
  const bytes = new Uint8Array(binary.length);
  for (let i = 0; i < binary.length; i++) {
    bytes[i] = binary.charCodeAt(i);
  }
  return bytes.buffer;
}

export interface PasskeyCredential {
  id: string;
  name: string;
  created_at: string;
  last_used_at: string | null;
  platform: string;
}

export interface PasskeyLoginResult {
  token: string;
  user: {
    id: string;
    email: string;
    created_at: string;
    updated_at: string;
  };
  expires_at: string;
}

class PasskeyService {
  isSupported(): boolean {
    return Passkey.isSupported();
  }

  getPlatformInfo(): { os: string; minVersion: string } {
    if (Platform.OS === 'ios') {
      return { os: 'iOS', minVersion: '15.0' };
    }
    return { os: 'Android', minVersion: 'API 28' };
  }

  async loginWithPasskey(email?: string): Promise<PasskeyLoginResult> {
    try {
      // Get challenge from server
      const options = email
        ? await api.webauthnLoginBegin(email)
        : await api.webauthnLoginBeginDiscoverable();

      // Format options for react-native-passkey
      const requestJson = JSON.stringify({
        challenge: options.publicKey.challenge,
        rpId: options.publicKey.rpId,
        allowCredentials: options.publicKey.allowCredentials?.map((cred: any) => ({
          id: cred.id,
          type: cred.type,
          transports: cred.transports,
        })),
        userVerification: options.publicKey.userVerification || 'preferred',
        timeout: options.publicKey.timeout || 60000,
      });

      // Authenticate with passkey
      const result: PasskeyGetResult = await Passkey.get(requestJson);

      // Send assertion to server
      const credential = {
        id: result.id,
        rawId: result.rawId,
        type: 'public-key',
        response: {
          authenticatorData: result.response.authenticatorData,
          clientDataJSON: result.response.clientDataJSON,
          signature: result.response.signature,
          userHandle: result.response.userHandle,
        },
      };

      const loginResult = await api.webauthnLoginFinish(credential);
      return loginResult;
    } catch (error: any) {
      if (error.message?.includes('cancelled') || error.message?.includes('canceled')) {
        throw new Error('Passkey authentication was cancelled');
      }
      if (error.message?.includes('not found') || error.message?.includes('No credentials')) {
        throw new Error('No passkey found for this account');
      }
      throw new Error(error.message || 'Passkey authentication failed');
    }
  }

  async registerPasskey(): Promise<PasskeyCredential> {
    try {
      // Get registration options from server
      const options = await api.webauthnRegisterBegin();

      // Format options for react-native-passkey
      const requestJson = JSON.stringify({
        challenge: options.publicKey.challenge,
        rp: {
          id: options.publicKey.rp.id,
          name: options.publicKey.rp.name,
        },
        user: {
          id: options.publicKey.user.id,
          name: options.publicKey.user.name,
          displayName: options.publicKey.user.displayName,
        },
        pubKeyCredParams: options.publicKey.pubKeyCredParams,
        authenticatorSelection: {
          authenticatorAttachment: 'platform',
          residentKey: 'preferred',
          userVerification: 'preferred',
        },
        timeout: options.publicKey.timeout || 60000,
        attestation: options.publicKey.attestation || 'none',
        excludeCredentials: options.publicKey.excludeCredentials?.map((cred: any) => ({
          id: cred.id,
          type: cred.type,
        })),
      });

      // Create passkey
      const result: PasskeyCreateResult = await Passkey.create(requestJson);

      // Send attestation to server
      const credential = {
        id: result.id,
        rawId: result.rawId,
        type: 'public-key',
        response: {
          attestationObject: result.response.attestationObject,
          clientDataJSON: result.response.clientDataJSON,
        },
      };

      const registrationResult = await api.webauthnRegisterFinish(credential);
      return registrationResult;
    } catch (error: any) {
      if (error.message?.includes('cancelled') || error.message?.includes('canceled')) {
        throw new Error('Passkey registration was cancelled');
      }
      if (error.message?.includes('already registered') || error.message?.includes('exists')) {
        throw new Error('This passkey is already registered');
      }
      throw new Error(error.message || 'Passkey registration failed');
    }
  }

  async getCredentials(): Promise<PasskeyCredential[]> {
    try {
      const credentials = await api.getWebauthnCredentials();
      return credentials;
    } catch (error: any) {
      throw new Error(error.message || 'Failed to fetch passkeys');
    }
  }

  async renameCredential(id: string, name: string): Promise<void> {
    try {
      await api.renameWebauthnCredential(id, name);
    } catch (error: any) {
      throw new Error(error.message || 'Failed to rename passkey');
    }
  }

  async deleteCredential(id: string): Promise<void> {
    try {
      await api.deleteWebauthnCredential(id);
    } catch (error: any) {
      throw new Error(error.message || 'Failed to delete passkey');
    }
  }
}

export const passkeyService = new PasskeyService();
export default passkeyService;
