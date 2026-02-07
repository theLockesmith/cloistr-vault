import React, { useState } from 'react';
import {
  View,
  StyleSheet,
  ScrollView,
  KeyboardAvoidingView,
  Platform,
} from 'react-native';
import {
  Text,
  TextInput,
  Button,
  Card,
  SegmentedButtons,
  useTheme,
  Surface,
} from 'react-native-paper';
import { useForm, Controller } from 'react-hook-form';
import { useNavigation } from '@react-navigation/native';
import { StackNavigationProp } from '@react-navigation/stack';
import Icon from 'react-native-vector-icons/MaterialCommunityIcons';
import Toast from 'react-native-toast-message';

import { useAuth } from '../../contexts/AuthContext';
import { AuthStackParamList } from '../../navigation/AppNavigator';
import BiometricAuth from '../../components/BiometricAuth';

type LoginScreenNavigationProp = StackNavigationProp<AuthStackParamList, 'Login'>;

interface LoginForm {
  email: string;
  password: string;
}

const LoginScreen: React.FC = () => {
  const navigation = useNavigation<LoginScreenNavigationProp>();
  const { login, loginWithNostr, loading } = useAuth();
  const theme = useTheme();
  const [authMethod, setAuthMethod] = useState('email');
  const [nostrPublicKey, setNostrPublicKey] = useState('');
  const [passwordVisible, setPasswordVisible] = useState(false);

  const {
    control,
    handleSubmit,
    formState: { errors },
  } = useForm<LoginForm>();

  const onEmailSubmit = async (data: LoginForm) => {
    try {
      await login(data.email, data.password);
    } catch (error) {
      // Error handling is done in AuthContext
    }
  };

  const onNostrLogin = async () => {
    if (!nostrPublicKey || nostrPublicKey.length !== 64) {
      Toast.show({
        type: 'error',
        text1: 'Invalid Nostr Key',
        text2: 'Please enter a valid 64-character Nostr public key',
      });
      return;
    }

    try {
      Toast.show({
        type: 'error',
        text1: 'Feature Coming Soon',
        text2: 'Nostr login requires integration with Nostr signing libraries',
      });
    } catch (error) {
      // Error handling is done in AuthContext
    }
  };

  const handleBiometricSuccess = () => {
    // In a real implementation, this would log in with stored credentials
    Toast.show({
      type: 'success',
      text1: 'Biometric Authentication',
      text2: 'Feature coming soon - will auto-fill saved credentials',
    });
  };

  const authMethodOptions = [
    { value: 'email', label: 'Email' },
    { value: 'nostr', label: 'Nostr' },
  ];

  return (
    <KeyboardAvoidingView
      style={styles.container}
      behavior={Platform.OS === 'ios' ? 'padding' : 'height'}
    >
      <ScrollView contentContainerStyle={styles.scrollContainer}>
        <Surface style={[styles.header, { backgroundColor: theme.colors.primary }]}>
          <Icon name="shield-key" size={48} color={theme.colors.onPrimary} />
          <Text variant="headlineMedium" style={[styles.title, { color: theme.colors.onPrimary }]}>
            Coldforge Vault
          </Text>
          <Text variant="bodyMedium" style={[styles.subtitle, { color: theme.colors.onPrimary }]}>
            Zero-knowledge password manager
          </Text>
        </Surface>

        <View style={styles.content}>
          <Card style={styles.card}>
            <Card.Content style={styles.cardContent}>
              {/* Biometric Authentication */}
              <BiometricAuth
                onSuccess={handleBiometricSuccess}
                style={styles.biometricButton}
              />

              {/* Auth Method Toggle */}
              <SegmentedButtons
                value={authMethod}
                onValueChange={setAuthMethod}
                buttons={authMethodOptions}
                style={styles.segmentedButtons}
              />

              {authMethod === 'email' ? (
                <View style={styles.formContainer}>
                  <Controller
                    control={control}
                    rules={{
                      required: 'Email is required',
                      pattern: {
                        value: /\S+@\S+\.\S+/,
                        message: 'Please enter a valid email',
                      },
                    }}
                    render={({ field: { onChange, onBlur, value } }) => (
                      <TextInput
                        mode="outlined"
                        label="Email Address"
                        placeholder="Enter your email"
                        value={value}
                        onBlur={onBlur}
                        onChangeText={onChange}
                        error={!!errors.email}
                        keyboardType="email-address"
                        autoCapitalize="none"
                        autoCorrect={false}
                        left={<TextInput.Icon icon="email" />}
                        style={styles.input}
                      />
                    )}
                    name="email"
                  />
                  {errors.email && (
                    <Text variant="bodySmall" style={styles.errorText}>
                      {errors.email.message}
                    </Text>
                  )}

                  <Controller
                    control={control}
                    rules={{
                      required: 'Password is required',
                      minLength: {
                        value: 8,
                        message: 'Password must be at least 8 characters',
                      },
                    }}
                    render={({ field: { onChange, onBlur, value } }) => (
                      <TextInput
                        mode="outlined"
                        label="Password"
                        placeholder="Enter your password"
                        value={value}
                        onBlur={onBlur}
                        onChangeText={onChange}
                        error={!!errors.password}
                        secureTextEntry={!passwordVisible}
                        left={<TextInput.Icon icon="lock" />}
                        right={
                          <TextInput.Icon
                            icon={passwordVisible ? 'eye-off' : 'eye'}
                            onPress={() => setPasswordVisible(!passwordVisible)}
                          />
                        }
                        style={styles.input}
                      />
                    )}
                    name="password"
                  />
                  {errors.password && (
                    <Text variant="bodySmall" style={styles.errorText}>
                      {errors.password.message}
                    </Text>
                  )}

                  <Button
                    mode="contained"
                    onPress={handleSubmit(onEmailSubmit)}
                    loading={loading}
                    disabled={loading}
                    style={styles.submitButton}
                    contentStyle={styles.buttonContent}
                  >
                    {loading ? 'Signing in...' : 'Sign In'}
                  </Button>
                </View>
              ) : (
                <View style={styles.formContainer}>
                  <TextInput
                    mode="outlined"
                    label="Nostr Public Key"
                    placeholder="Enter your 64-character Nostr public key"
                    value={nostrPublicKey}
                    onChangeText={setNostrPublicKey}
                    maxLength={64}
                    multiline
                    numberOfLines={3}
                    left={<TextInput.Icon icon="key-variant" />}
                    style={[styles.input, styles.nostrKeyInput]}
                  />
                  <Text variant="bodySmall" style={styles.helperText}>
                    Your public key should be 64 hexadecimal characters
                  </Text>

                  <Card style={styles.nostrInfoCard}>
                    <Card.Content>
                      <View style={styles.nostrInfoHeader}>
                        <Icon name="information" size={20} color={theme.colors.primary} />
                        <Text variant="titleSmall" style={styles.nostrInfoTitle}>
                          Nostr Integration
                        </Text>
                      </View>
                      <Text variant="bodySmall" style={styles.nostrInfoText}>
                        • Your vault is encrypted using your Nostr identity
                        {'\n'}• Requires a Nostr client for authentication
                        {'\n'}• Currently in development - coming soon
                      </Text>
                    </Card.Content>
                  </Card>

                  <Button
                    mode="contained"
                    onPress={onNostrLogin}
                    loading={loading}
                    disabled={loading || !nostrPublicKey}
                    style={styles.submitButton}
                    contentStyle={styles.buttonContent}
                  >
                    {loading ? 'Signing in...' : 'Sign In with Nostr'}
                  </Button>
                </View>
              )}

              <Button
                mode="text"
                onPress={() => navigation.navigate('Register')}
                style={styles.registerButton}
              >
                Don't have an account? Create one
              </Button>
            </Card.Content>
          </Card>

          <Text variant="bodySmall" style={styles.securityNote}>
            Your data is encrypted locally and never stored unencrypted on our servers
          </Text>
        </View>
      </ScrollView>
    </KeyboardAvoidingView>
  );
};

const styles = StyleSheet.create({
  container: {
    flex: 1,
    backgroundColor: '#f8fafc',
  },
  scrollContainer: {
    flexGrow: 1,
  },
  header: {
    alignItems: 'center',
    paddingVertical: 48,
    paddingHorizontal: 24,
  },
  title: {
    marginTop: 16,
    fontWeight: '600',
  },
  subtitle: {
    marginTop: 8,
    opacity: 0.9,
  },
  content: {
    flex: 1,
    paddingHorizontal: 24,
    paddingTop: 24,
  },
  card: {
    elevation: 4,
    marginBottom: 24,
  },
  cardContent: {
    paddingVertical: 24,
  },
  biometricButton: {
    marginBottom: 24,
  },
  segmentedButtons: {
    marginBottom: 24,
  },
  formContainer: {
    gap: 16,
  },
  input: {
    backgroundColor: 'transparent',
  },
  nostrKeyInput: {
    minHeight: 80,
  },
  errorText: {
    color: '#dc2626',
    marginTop: -12,
  },
  helperText: {
    color: '#64748b',
    marginTop: -12,
  },
  nostrInfoCard: {
    backgroundColor: '#f1f5f9',
    marginVertical: 8,
  },
  nostrInfoHeader: {
    flexDirection: 'row',
    alignItems: 'center',
    marginBottom: 8,
  },
  nostrInfoTitle: {
    marginLeft: 8,
    fontWeight: '600',
  },
  nostrInfoText: {
    color: '#475569',
    lineHeight: 18,
  },
  submitButton: {
    marginTop: 8,
  },
  buttonContent: {
    height: 48,
  },
  registerButton: {
    marginTop: 16,
  },
  securityNote: {
    textAlign: 'center',
    color: '#64748b',
    paddingHorizontal: 16,
  },
});

export default LoginScreen;