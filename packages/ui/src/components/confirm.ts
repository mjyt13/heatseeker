import { Alert, Platform } from 'react-native';

export interface ConfirmOptions {
  title: string;
  message?: string;
  confirmText: string;
  cancelText: string;
  destructive?: boolean;
}

/**
 * Asks the user to confirm an action. Native uses Alert; react-native-web's
 * Alert does nothing, so the web falls back to the browser dialog.
 */
export function confirm({
  title,
  message,
  confirmText,
  cancelText,
  destructive,
}: ConfirmOptions): Promise<boolean> {
  if (Platform.OS === 'web') {
    const text = message ? `${title}\n\n${message}` : title;
    return Promise.resolve(
      typeof globalThis.confirm === 'function' ? globalThis.confirm(text) : true,
    );
  }
  return new Promise((resolve) => {
    Alert.alert(
      title,
      message,
      [
        { text: cancelText, style: 'cancel', onPress: () => resolve(false) },
        {
          text: confirmText,
          style: destructive ? 'destructive' : 'default',
          onPress: () => resolve(true),
        },
      ],
      { cancelable: true, onDismiss: () => resolve(false) },
    );
  });
}
