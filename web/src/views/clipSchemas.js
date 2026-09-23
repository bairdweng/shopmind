// 剪辑工具的输入声明：ToolShell 按这里的字段动态生成表单。
// 新增工具只需两处：后端 internal/clip/xxx.go 插件文件 + 这里加一项 schema。
//
// 字段类型：
//   file     el-upload 拖拽上传，提交为 FormData 文件
//   textarea el-input 多行文本
//   text     el-input 单行文本
//   number   el-input-number 数字
//   radio    el-radio-group 按钮组
//
// 字段可加 show: (inputs) => bool，依赖其它字段值条件显示。
export const SCHEMAS = {
  text2srt: {
    submitText: '生成 SRT',
    fields: [
      {
        key: 'audio',
        type: 'file',
        label: '音频文件',
        accept: '.mp3,.wav,.m4a,.mp4,audio/*,video/mp4',
        required: true,
        hint: '拖入或点击选择 mp3 / wav / m4a / mp4'
      },
      {
        key: 'text',
        type: 'textarea',
        label: '字幕文本',
        rows: 10,
        required: true,
        placeholder: '一行一句，例如：\n大家好，欢迎来到本期课程。\n今天我们讲三个要点。',
        hint: '一行一句。文案与口播一致才能对准；先改稿再对齐。'
      },
      {
        key: 'language',
        type: 'radio',
        label: '语言',
        default: 'zh',
        options: [
          { value: 'zh', label: '中文' },
          { value: 'en', label: '英文' }
        ]
      }
    ]
  },
  plain2srt: {
    submitText: '生成 SRT',
    fields: [
      {
        key: 'text',
        type: 'textarea',
        label: '字幕文本',
        rows: 10,
        required: true,
        placeholder: '一行一句，例如：\n大家好，欢迎来到本期课程。\n今天我们讲三个要点。',
        hint: '一行一句。先排草稿，后续有音频可用「文本转字幕」精确对齐。'
      },
      {
        key: 'mode',
        type: 'radio',
        label: '计时方式',
        default: 'speed',
        options: [
          { value: 'speed', label: '按语速估算' },
          { value: 'total', label: '指定总时长' },
          { value: 'fixed', label: '固定时长' }
        ]
      },
      {
        key: 'wpm',
        type: 'number',
        label: '语速(字/分)',
        default: 240,
        min: 60,
        max: 600,
        step: 10,
        show: (inputs) => inputs.mode === 'speed'
      },
      {
        key: 'total',
        type: 'number',
        label: '总时长(秒)',
        default: 300,
        min: 1,
        step: 1,
        show: (inputs) => inputs.mode === 'total'
      },
      {
        key: 'fixed_each',
        type: 'number',
        label: '每条秒数',
        default: 3,
        min: 0.5,
        step: 0.5,
        show: (inputs) => inputs.mode === 'fixed'
      },
      {
        key: 'start',
        type: 'number',
        label: '起始偏移(秒)',
        default: 0,
        min: 0,
        step: 0.5,
        hint: '从第几秒开始，前面有片头时使用'
      },
      {
        key: 'gap',
        type: 'number',
        label: '条间隔(秒)',
        default: 0.1,
        min: 0,
        step: 0.1
      }
    ]
  }
}
