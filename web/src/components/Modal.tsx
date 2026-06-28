import React from 'react';
import { createPortal } from 'react-dom';
import { X } from 'lucide-react';

interface ModalProps {
  title: string;
  onClose: () => void;
  children: React.ReactNode;
  maxWidth?: number;
}

export function Modal({ title, onClose, children, maxWidth = 640 }: ModalProps) {
  return createPortal(
    <div className="fixed inset-0 z-[320] flex items-center justify-center p-4 bg-black/60 backdrop-blur-[4px] animate-fade-in">
      <div 
        className="relative flex flex-col w-full bg-surface border border-border shadow-2xl rounded-2xl animate-scale-in" 
        style={{ maxWidth }}
      >
        <div className="flex items-center justify-between px-6 py-4 border-b border-border bg-black/5 dark:bg-white/5 rounded-t-2xl">
          <h2 className="text-[17px] font-bold text-textPrimary m-0 tracking-tight">{title}</h2>
          <button 
            type="button" 
            onClick={onClose} 
            className="flex items-center justify-center w-8 h-8 rounded-full text-textSecondary hover:text-textPrimary hover:bg-black/10 dark:hover:bg-white/10 transition-colors"
          >
            <X size={18} />
          </button>
        </div>
        {children}
      </div>
    </div>,
    document.body
  );
}
