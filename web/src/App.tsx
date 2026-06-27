import { useState, useEffect } from 'react';
import { Routes, Route, Navigate, useNavigate, NavLink } from 'react-router-dom';
import { getAuthToken, setAuthToken } from './utils';
import { api } from './api';

// Icons
import { LayoutDashboard, Globe, Users, History, Settings as SettingsIcon, LogOut, Search, Moon, Sun } from 'lucide-react';

// Views
import Dashboard from './views/Dashboard';
import Sites from './views/Sites';
import Accounts from './views/Accounts';
import Logs from './views/Logs';
import Settings from './views/Settings';

const LogoIcon = ({ size = 32 }: { size?: number }) => (
  <svg width={size} height={size} viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
    <defs>
      <linearGradient id="aggr-gradient" x1="0%" y1="0%" x2="100%" y2="100%">
        <stop offset="0%" stopColor="#4f46e5" />
        <stop offset="100%" stopColor="#06b6d4" />
      </linearGradient>
    </defs>
    <circle cx="12" cy="12" r="3.5" fill="url(#aggr-gradient)" stroke="none" />
    <circle cx="12" cy="12" r="6" stroke="url(#aggr-gradient)" strokeWidth="1.2" strokeDasharray="2 2" opacity={0.6} />
    <circle cx="12" cy="12" r="9" stroke="url(#aggr-gradient)" strokeWidth="1.2" strokeDasharray="3 3" opacity={0.4} />
    
    <circle cx="6" cy="6" r="1.2" fill="url(#aggr-gradient)" stroke="none" />
    <path d="M6.8 6.8C8.5 8.5 10 10 12 12" stroke="url(#aggr-gradient)" strokeWidth="1.5" />
    
    <circle cx="18" cy="6" r="1.2" fill="url(#aggr-gradient)" stroke="none" />
    <path d="M17.2 6.8C15.5 8.5 14 10 12 12" stroke="url(#aggr-gradient)" strokeWidth="1.5" />
    
    <circle cx="6" cy="18" r="1.2" fill="url(#aggr-gradient)" stroke="none" />
    <path d="M6.8 17.2C8.5 15.5 10 14 12 12" stroke="url(#aggr-gradient)" strokeWidth="1.5" />
    
    <circle cx="18" cy="18" r="1.2" fill="url(#aggr-gradient)" stroke="none" />
    <path d="M17.2 17.2C15.5 15.5 14 14 12 12" stroke="url(#aggr-gradient)" strokeWidth="1.5" />
  </svg>
);

function Login() {
  const [token, setToken] = useState('');
  const [error, setError] = useState('');
  const [loading, setLoading] = useState(false);
  const navigate = useNavigate();

  const handleLogin = async (e: React.FormEvent) => {
    e.preventDefault();
    setError('');
    setLoading(true);
    try {
      setAuthToken(token);
      await api.get('/api/platforms');
      navigate('/');
    } catch (err: any) {
      setError('无效的 Token 或服务器错误');
      setAuthToken('');
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="flex flex-col items-center justify-center min-h-screen p-4 bg-background">
      <div className="flex flex-col md:flex-row w-full max-w-[880px] bg-surface rounded-2xl shadow-xl overflow-hidden animate-scale-in border border-border">
        {/* Brand Panel */}
        <section className="hidden md:flex flex-col justify-between w-1/2 p-12 bg-gradient-to-br from-primary/10 to-info/10 border-r border-border">
          <div>
            <div className="flex items-center justify-center w-16 h-16 bg-surface rounded-2xl shadow-sm border border-white/20 mb-8">
              <LogoIcon size={40} />
            </div>
            <div className="text-[32px] font-bold text-textPrimary tracking-tight mb-2">AggrSite</div>
            <div className="text-[13px] font-bold text-primary tracking-[0.2em] uppercase mb-6">统一 API 聚合器</div>
            <p className="text-[15px] text-textSecondary leading-relaxed max-w-[280px]">
              无缝管理您的账户、API 密钥并追踪多个平台的使用情况。
            </p>
          </div>
          <div className="text-[12px] text-textMuted font-medium">
            &copy; {new Date().getFullYear()} Metapi. All rights reserved.
          </div>
        </section>

        {/* Auth Panel */}
        <section className="flex flex-col justify-center w-full md:w-1/2 p-8 sm:p-12 lg:p-16">
          <div className="text-[12px] font-bold text-primary tracking-[0.15em] uppercase mb-4">管理员门户</div>
          <h2 className="text-[28px] font-bold text-textPrimary tracking-tight mb-2">登录</h2>
          <p className="text-[14px] text-textSecondary mb-8">请输入您的管理员 Token 以继续。</p>
          
          <form onSubmit={handleLogin} className="flex flex-col gap-5">
            <div>
              <label className="block text-[13px] font-semibold text-textPrimary mb-2" htmlFor="admin-token-input">
                管理员 Token
              </label>
              <input
                id="admin-token-input"
                type="password"
                placeholder="••••••••••••••••"
                className="w-full px-4 py-3 bg-background border border-border rounded-xl text-[14px] text-textPrimary placeholder:text-textMuted focus:outline-none focus:border-primary focus:ring-4 focus:ring-primary/10 transition-all font-mono"
                value={token}
                onChange={(e) => setToken(e.target.value)}
                autoFocus
              />
            </div>
            {error && (
              <div className="px-4 py-3 bg-dangerSoft text-danger rounded-xl text-[13px] font-medium animate-shake border border-danger/20">
                {error}
              </div>
            )}
            <button
              type="submit"
              disabled={loading || !token}
              className="relative flex items-center justify-center w-full px-4 py-3 mt-2 text-[14px] font-medium text-white bg-primary rounded-xl transition-all duration-200 hover:bg-primaryHover hover:-translate-y-px hover:shadow-lg hover:shadow-primary/25 active:scale-[0.98] disabled:opacity-50 disabled:cursor-not-allowed disabled:transform-none disabled:shadow-none"
            >
              {loading ? <span className="w-5 h-5 border-2 border-white/30 border-t-white rounded-full animate-spin" /> : '进入系统'}
            </button>
          </form>
        </section>
      </div>
    </div>
  );
}

function Layout({ children }: { children: React.ReactNode }) {
  const navigate = useNavigate();
  const [themeMode, setThemeMode] = useState<'light' | 'dark'>(() => {
    return (localStorage.getItem('theme') as 'light' | 'dark') || 'light';
  });

  useEffect(() => {
    document.documentElement.setAttribute('data-theme', themeMode);
    localStorage.setItem('theme', themeMode);
  }, [themeMode]);

  const navItems = [
    { path: '/', label: '仪表盘', icon: LayoutDashboard },
    { path: '/sites', label: '站点', icon: Globe },
    { path: '/accounts', label: '账户', icon: Users },
    { path: '/logs', label: '日志', icon: History },
    { path: '/settings', label: '设置', icon: SettingsIcon },
  ];

  const handleLogout = () => {
    localStorage.removeItem('aggrsite_token');
    navigate('/login');
  };

  const toggleTheme = () => {
    setThemeMode(prev => prev === 'light' ? 'dark' : 'light');
  };

  const iconBtnClass = "w-8 h-8 flex items-center justify-center rounded-lg text-textSecondary hover:text-textPrimary hover:bg-black/5 dark:hover:bg-white/10 transition-colors";

  return (
    <div className="min-h-screen bg-background flex flex-col">
      <header className="fixed top-0 left-0 right-0 h-14 bg-surface/80 backdrop-blur-lg border-b border-border z-40 flex items-center justify-between px-4 sm:px-6">
        <div className="flex items-center gap-3">
          <LogoIcon size={26} />
          <span className="font-bold text-[16px] tracking-tight text-textPrimary hidden sm:block">AggrSite</span>
        </div>
        
        <div className="flex items-center gap-2 sm:gap-3">
          <button className="hidden sm:flex items-center gap-2 px-3 py-1.5 bg-background border border-border rounded-lg text-textMuted hover:text-textPrimary hover:border-borderLight transition-colors w-48 lg:w-64">
            <Search size={14} />
            <span className="text-[13px] flex-1 text-left">搜索...</span>
            <kbd className="hidden lg:inline-flex items-center px-1.5 h-5 bg-surface border border-border rounded text-[10px] font-mono font-medium text-textMuted">Ctrl K</kbd>
          </button>
          
          <div className="w-[1px] h-5 bg-border mx-1 hidden sm:block" />
          
          <button
            className={iconBtnClass}
            onClick={toggleTheme}
            aria-label="切换主题"
          >
            {themeMode === 'light' ? <Moon size={18} /> : <Sun size={18} />}
          </button>

          <button onClick={handleLogout} className="w-8 h-8 flex items-center justify-center rounded-lg text-danger/80 hover:text-danger hover:bg-dangerSoft transition-colors" title="退出登录">
            <LogOut size={18} />
          </button>
        </div>
      </header>

      <div className="flex flex-1 pt-14">
        <aside className="fixed left-0 top-14 bottom-0 w-[64px] lg:w-[240px] bg-surface border-r border-border overflow-y-auto z-30 transition-all duration-300">
          <div className="p-3 lg:p-4 flex flex-col gap-1.5">
            <div className="hidden lg:block px-3 py-2 text-[11px] font-bold text-textMuted uppercase tracking-wider mb-1">
              控制台
            </div>
            {navItems.map(item => {
              const Icon = item.icon;
              return (
                <NavLink 
                  key={item.path} 
                  to={item.path} 
                  className={({ isActive }) => 
                    `flex items-center justify-center lg:justify-start gap-3 p-3 lg:px-3 lg:py-2.5 rounded-xl transition-all ${
                      isActive 
                        ? 'bg-primary/10 text-primary font-semibold' 
                        : 'text-textSecondary hover:text-textPrimary hover:bg-black/5 dark:hover:bg-white/5 font-medium'
                    }`
                  }
                  title={item.label}
                >
                  <Icon size={20} />
                  <span className="hidden lg:block text-[13.5px]">{item.label}</span>
                </NavLink>
              );
            })}
          </div>
        </aside>

        <main className="flex-1 ml-[64px] lg:ml-[240px] p-4 sm:p-6 lg:p-8 transition-all duration-300">
          <div className="max-w-[1200px] mx-auto w-full">
            {children}
          </div>
        </main>
      </div>
    </div>
  );
}

function ProtectedRoute({ children }: { children: React.ReactNode }) {
  const token = getAuthToken();
  if (!token) return <Navigate to="/login" />;
  return <Layout>{children}</Layout>;
}

function App() {
  return (
    <Routes>
      <Route path="/login" element={<Login />} />
      <Route path="/" element={<ProtectedRoute><Dashboard /></ProtectedRoute>} />
      <Route path="/sites" element={<ProtectedRoute><Sites /></ProtectedRoute>} />
      <Route path="/accounts" element={<ProtectedRoute><Accounts /></ProtectedRoute>} />
      <Route path="/logs" element={<ProtectedRoute><Logs /></ProtectedRoute>} />
      <Route path="/settings" element={<ProtectedRoute><Settings /></ProtectedRoute>} />
      <Route path="*" element={<Navigate to="/" />} />
    </Routes>
  );
}

export default App;
