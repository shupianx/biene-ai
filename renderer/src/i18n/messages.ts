export const messages = {
  en: {
    common: {
      back: 'Back',
      cancel: 'Cancel',
      close: 'Close',
      confirm: 'Confirm',
      create: 'Create',
      delete: 'Delete',
      errorLabel: 'Error',
      more: 'More',
      save: 'Save',
      settings: 'Settings',
    },
    titleBar: {
      about: 'About Biene',
      context: 'Workspace',
      darkMode: 'Dark mode',
      openSettingsMenu: 'Open settings menu',
    },
    grid: {
      createOne: 'Create one',
      deleteAgentMessage: 'Delete {name}? Its workspace and stored history will be removed from disk.',
      deleteAgentTitle: 'Delete Agent',
      newAgent: 'New Agent',
      noAgentsYet: 'No agents yet',
    },
    agent: {
      anotherAgent: 'another agent',
      loading: 'Loading agent…',
      notFound: 'Agent not found.',
      ready: 'Agent ready. Send a message to start.',
      workingDirectory: 'Working directory',
    },
    input: {
      interrupted: 'Interrupted.',
      interrupt: 'Interrupt',
      interrupting: 'Interrupting',
      placeholder: 'Message this agent…',
      send: 'Send message',
    },
    message: {
      from: 'From',
    },
    agentName: {
      defaultName: 'Agent {index}',
      exists: 'Agent name already exists.',
      label: 'Agent name',
    },
    modal: {
      advancedSettings: 'Advanced settings',
      agentSettingsTitle: 'Agent Settings',
      customInstructions: 'Custom instructions',
      customInstructionsPlaceholder: 'Optional agent-specific instructions',
      newAgentTitle: 'New Agent',
      profile: 'Profile',
      style: 'Style',
      domain: 'Domain',
      toolPermissions: 'Tool permissions',
      toolPermissionsHint: 'Closed means the agent will ask before using that permission group.',
    },
    permissions: {
      approvalDescription: 'This agent wants to use a protected capability.',
      allowAlways: 'Always Allow',
      allowOnce: 'Allow Once',
      deny: 'Deny',
      send_to_agent: {
        description: 'Allows sending messages or files to other agents.',
        label: 'Agent transfer',
      },
      write: {
        description: 'Allows both Write and Edit tool calls.',
        label: 'File changes',
      },
    },
    profile: {
      domain: {
        coding: {
          description: 'Software engineering work inside the workspace.',
          label: 'Coding',
        },
        general: {
          description: 'General problem-solving, planning, and analysis.',
          label: 'General',
        },
      },
      style: {
        balanced: {
          description: 'Balanced speed, clarity, and completeness.',
          label: 'Balanced',
        },
        concise: {
          description: 'Shorter, tighter responses with minimal preamble.',
          label: 'Concise',
        },
        proactive: {
          description: 'Pushes the task forward aggressively when the next step is clear.',
          label: 'Proactive',
        },
        skeptical: {
          description: 'More verification, risk checking, and challenge of assumptions.',
          label: 'Skeptical',
        },
        thorough: {
          description: 'More complete explanations with assumptions and tradeoffs.',
          label: 'Thorough',
        },
      },
    },
    sessionStatus: {
      approval: 'Needs Approval',
      error: 'Error',
      idle: 'Idle',
      running: 'Running',
    },
    time: {
      yesterdayAt: 'Yesterday {time}',
    },
    tool: {
      input: 'Input',
      output: 'Output',
    },
  },
  'zh-CN': {
    common: {
      back: '返回',
      cancel: '取消',
      close: '关闭',
      confirm: '确认',
      create: '创建',
      delete: '删除',
      errorLabel: '错误',
      more: '更多',
      save: '保存',
      settings: '设置',
    },
    titleBar: {
      about: '关于 Biene',
      context: 'Workspace',
      darkMode: '深色模式',
      openSettingsMenu: '打开设置菜单',
    },
    grid: {
      createOne: '创建一个',
      deleteAgentMessage: '删除 {name}？它的工作区和历史记录都会从磁盘移除。',
      deleteAgentTitle: '删除智能体',
      newAgent: '新建实例',
      noAgentsYet: '还没有智能体',
    },
    agent: {
      anotherAgent: '另一个智能体',
      loading: '正在加载智能体…',
      notFound: '未找到智能体。',
      ready: '智能体已就绪，发送一条消息开始吧。',
      workingDirectory: '工作目录',
    },
    input: {
      interrupted: '已中断。',
      interrupt: '中断',
      interrupting: '正在中断',
      placeholder: '向这个智能体发送消息…',
      send: '发送消息',
    },
    message: {
      from: '来自',
    },
    agentName: {
      defaultName: '智能体 {index}',
      exists: '智能体名称已存在。',
      label: '智能体名称',
    },
    modal: {
      advancedSettings: '高级设置',
      agentSettingsTitle: '智能体设置',
      customInstructions: '自定义指令',
      customInstructionsPlaceholder: '可选的智能体专属指令',
      newAgentTitle: '新建智能体',
      profile: '配置',
      style: '风格',
      domain: '领域',
      toolPermissions: '工具权限',
      toolPermissionsHint: '关闭后，智能体在使用该权限组前会先请求确认。',
    },
    permissions: {
      approvalDescription: '这个智能体想要使用受保护的能力。',
      allowAlways: '始终允许',
      allowOnce: '允许一次',
      deny: '拒绝',
      send_to_agent: {
        description: '允许向其他智能体发送消息或文件',
        label: '智能体协作',
      },
      write: {
        description: '允许编辑文件',
        label: '文件修改',
      },
    },
    profile: {
      domain: {
        coding: {
          description: '在工作区内处理软件工程任务。',
          label: '编程',
        },
        general: {
          description: '通用问题解决、规划与分析。',
          label: '通用',
        },
      },
      style: {
        balanced: {
          description: '在速度、清晰度和完整性之间取得平衡。',
          label: '平衡',
        },
        concise: {
          description: '回答更短、更直接，减少铺垫。',
          label: '简洁',
        },
        proactive: {
          description: '当下一步明确时更主动地推进任务。',
          label: '主动',
        },
        skeptical: {
          description: '更重视验证、风险检查和质疑假设。',
          label: '审慎',
        },
        thorough: {
          description: '解释更完整，更多覆盖假设和权衡。',
          label: '详尽',
        },
      },
    },
    sessionStatus: {
      approval: '待审批',
      error: '错误',
      idle: '空闲',
      running: '运行中',
    },
    time: {
      yesterdayAt: '昨天 {time}',
    },
    tool: {
      input: '输入',
      output: '输出',
    },
  },
} as const

export type AppLocale = keyof typeof messages
