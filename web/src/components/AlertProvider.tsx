import { createContext, useContext, useState, useEffect } from 'react';
import type { ReactNode } from 'react';
import { AlertModal } from './AlertModal';

interface AlertContextType {
  showAlert: (message: string) => void;
}

const AlertContext = createContext<AlertContextType | undefined>(undefined);

export function AlertProvider({ children }: { children: ReactNode }) {
  const [alertQueue, setAlertQueue] = useState<string[]>([]);

  const showAlert = (message: string) => {
    // Standardize newline formatting if needed
    setAlertQueue((prev) => [...prev, message]);
  };

  useEffect(() => {
    const originalAlert = window.alert;
    window.alert = (message: any) => {
      showAlert(String(message));
    };
    return () => {
      window.alert = originalAlert;
    };
  }, []);

  const handleClose = () => {
    setAlertQueue((prev) => prev.slice(1));
  };

  return (
    <AlertContext.Provider value={{ showAlert }}>
      {children}
      {alertQueue.length > 0 && (
        <AlertModal message={alertQueue[0]} onClose={handleClose} />
      )}
    </AlertContext.Provider>
  );
}

export function useAlert() {
  const context = useContext(AlertContext);
  if (!context) {
    throw new Error('useAlert must be used within an AlertProvider');
  }
  return context;
}
