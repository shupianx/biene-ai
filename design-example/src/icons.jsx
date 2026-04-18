// Minimal line icons, 1.5px stroke, industrial feel
const Icon = ({ d, size = 16, stroke = 1.5, fill = "none", children, style }) => (
  <svg width={size} height={size} viewBox="0 0 24 24" fill={fill} stroke="currentColor"
       strokeWidth={stroke} strokeLinecap="square" strokeLinejoin="miter"
       style={{ display: 'block', ...style }}>
    {d ? <path d={d} /> : children}
  </svg>
);

const IconPlus       = (p) => <Icon {...p}><path d="M12 5v14M5 12h14" /></Icon>;
const IconRefresh    = (p) => <Icon {...p}><path d="M3 12a9 9 0 0 1 15.5-6.3M21 4v5h-5M21 12a9 9 0 0 1-15.5 6.3M3 20v-5h5" /></Icon>;
const IconSettings   = (p) => <Icon {...p}><path d="M12 8.5a3.5 3.5 0 1 0 0 7 3.5 3.5 0 0 0 0-7Z M19.4 13.5l1.7 1.3-1.5 2.6-2-.7a7 7 0 0 1-1.7 1l-.3 2.1h-3l-.3-2.1a7 7 0 0 1-1.7-1l-2 .7-1.5-2.6 1.7-1.3a7 7 0 0 1 0-2l-1.7-1.3 1.5-2.6 2 .7a7 7 0 0 1 1.7-1l.3-2.1h3l.3 2.1a7 7 0 0 1 1.7 1l2-.7 1.5 2.6-1.7 1.3a7 7 0 0 1 0 2Z" /></Icon>;
const IconClose      = (p) => <Icon {...p}><path d="M5 5l14 14M19 5L5 19" /></Icon>;
const IconMenu       = (p) => <Icon {...p}><path d="M4 7h16M4 12h16M4 17h16" /></Icon>;
const IconGrid       = (p) => <Icon {...p}><path d="M4 4h7v7H4zM13 4h7v7h-7zM4 13h7v7H4zM13 13h7v7h-7z" /></Icon>;
const IconList       = (p) => <Icon {...p}><path d="M8 6h13M8 12h13M8 18h13M4 6h.01M4 12h.01M4 18h.01" /></Icon>;
const IconTerminal   = (p) => <Icon {...p}><path d="M4 5h16v14H4zM7 9l3 3-3 3M12 15h5" /></Icon>;
const IconPlay       = (p) => <Icon {...p}><path d="M7 5l12 7-12 7V5z" /></Icon>;
const IconPause      = (p) => <Icon {...p}><path d="M7 5h4v14H7zM13 5h4v14h-4z" /></Icon>;
const IconStop       = (p) => <Icon {...p}><path d="M6 6h12v12H6z" /></Icon>;
const IconArrow      = (p) => <Icon {...p}><path d="M5 12h14M13 6l6 6-6 6" /></Icon>;
const IconDot        = (p) => <Icon {...p}><circle cx="12" cy="12" r="4" fill="currentColor" stroke="none" /></Icon>;
const IconChip       = (p) => <Icon {...p}><path d="M6 6h12v12H6zM9 9h6v6H9zM9 2v3M15 2v3M9 19v3M15 19v3M2 9h3M2 15h3M19 9h3M19 15h3" /></Icon>;
const IconFolder     = (p) => <Icon {...p}><path d="M3 6h6l2 2h10v11H3z" /></Icon>;
const IconClock      = (p) => <Icon {...p}><path d="M12 6v6l4 2" /><circle cx="12" cy="12" r="9" fill="none" stroke="currentColor" strokeWidth="1.5" /></Icon>;
const IconMore       = (p) => <Icon {...p}><circle cx="6" cy="12" r="1" fill="currentColor" /><circle cx="12" cy="12" r="1" fill="currentColor" /><circle cx="18" cy="12" r="1" fill="currentColor" /></Icon>;
const IconCopy       = (p) => <Icon {...p}><path d="M9 9h10v10H9zM5 5h10v4M5 5v10h4" /></Icon>;
const IconTrash      = (p) => <Icon {...p}><path d="M4 7h16M9 7V4h6v3M6 7l1 13h10l1-13M10 11v6M14 11v6" /></Icon>;
const IconSearch     = (p) => <Icon {...p}><circle cx="11" cy="11" r="7" fill="none" stroke="currentColor" strokeWidth="1.5" /><path d="M20 20l-4-4" /></Icon>;
const IconBolt       = (p) => <Icon {...p}><path d="M13 3 5 14h6l-1 7 8-11h-6l1-7Z" /></Icon>;
const IconBranch     = (p) => <Icon {...p}><path d="M7 4v12M17 8v4a4 4 0 0 1-4 4H7" /><circle cx="7" cy="18" r="2" fill="none" stroke="currentColor" strokeWidth="1.5"/><circle cx="17" cy="6" r="2" fill="none" stroke="currentColor" strokeWidth="1.5"/></Icon>;

Object.assign(window, {
  Icon, IconPlus, IconRefresh, IconSettings, IconClose, IconMenu,
  IconGrid, IconList, IconTerminal, IconPlay, IconPause, IconStop,
  IconArrow, IconDot, IconChip, IconFolder, IconClock, IconMore,
  IconCopy, IconTrash, IconSearch, IconBolt, IconBranch,
});
