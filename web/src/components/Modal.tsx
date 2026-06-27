import React from 'react';
import { createPortal } from 'react-dom';
import { X } from 'lucide-react';

interface ModalProps {
  title: string;
  onClose: () => void;
  children: React.ReactNode;
  maxWidth?: number;
}

export function Modal({ title, onClose, children, maxWidth = 440 }: ModalProps) {
  return createPortal(
    <div className="modal-backdrop">
      <div className="modal-content animate-scale-in" style={{ width: '100%', maxWidth }}>
        <div className="modal-header">
          <h2 className="modal-title">{title}</h2>
          <button type="button" onClick={onClose} className="modal-close-button"><X size={20} /></button>
        </div>
        {children}
      </div>
    </div>,
    document.body
  );
}
