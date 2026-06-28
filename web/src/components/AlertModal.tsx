import { Modal } from './Modal';
import { AlertCircle, CheckCircle2, Info } from 'lucide-react';

interface AlertModalProps {
  message: string;
  onClose: () => void;
}

export function AlertModal({ message, onClose }: AlertModalProps) {
  // Determine style and icon based on message content
  let type: 'success' | 'danger' | 'info' = 'info';
  const lowercaseMsg = message.toLowerCase();

  if (
    lowercaseMsg.includes('错误') ||
    lowercaseMsg.includes('失败') ||
    lowercaseMsg.includes('error') ||
    lowercaseMsg.includes('failed') ||
    lowercaseMsg.includes('invalid') ||
    lowercaseMsg.includes('请求体格式错误') ||
    lowercaseMsg.includes('未找到')
  ) {
    type = 'danger';
  } else if (
    lowercaseMsg.includes('成功') ||
    lowercaseMsg.includes('完成') ||
    lowercaseMsg.includes('success') ||
    lowercaseMsg.includes('complete') ||
    lowercaseMsg.includes('正常')
  ) {
    type = 'success';
  }

  let icon = <Info className="w-6 h-6 text-info" />;
  let iconContainerClass = 'bg-infoSoft';

  if (type === 'danger') {
    icon = <AlertCircle className="w-6 h-6 text-danger" />;
    iconContainerClass = 'bg-dangerSoft';
  } else if (type === 'success') {
    icon = <CheckCircle2 className="w-6 h-6 text-success" />;
    iconContainerClass = 'bg-successSoft';
  }

  return (
    <Modal title="提示" onClose={onClose} maxWidth={400}>
      <div className="p-6 flex flex-col items-center text-center gap-4">
        <div className={`flex items-center justify-center w-12 h-12 rounded-full ${iconContainerClass} animate-scale-in`}>
          {icon}
        </div>
        <p className="text-[14.5px] text-textPrimary font-medium leading-relaxed whitespace-pre-wrap max-w-full">
          {message}
        </p>
      </div>
      <div className="flex items-center justify-center px-6 py-4 border-t border-border bg-black/5 dark:bg-white/5 rounded-b-2xl">
        <button
          type="button"
          onClick={onClose}
          className="w-full px-5 py-2.5 text-[13.5px] font-semibold text-white bg-primary rounded-xl transition-all duration-200 hover:bg-primaryHover hover:-translate-y-px hover:shadow-md hover:shadow-primary/10 active:scale-95"
        >
          确定
        </button>
      </div>
    </Modal>
  );
}
