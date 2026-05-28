# 手机与折叠屏适配方案

## 🎯 解决的问题

1. **浏览器UI遮挡**: Chrome、Safari等浏览器的地址栏和导航栏占用屏幕空间
2. **键盘弹起**: 软键盘弹起时遮挡输入框
3. **折叠屏适配**: 外屏/内屏切换、折叠线处理
4. **notch和安全区**: 刘海屏、圆角屏等非矩形屏幕的处理
5. **尺寸变化**: 浏览器窗口大小动态变化时的响应

---

## 📁 核心文件说明

### 1. `viewport-adapter.js` ⭐
**功能**: 动态视口适配的核心脚本

**工作原理**:
```javascript
// 使用 visualViewport API 获取实际可用高度
const usableHeight = window.visualViewport.height; // 不包含地址栏

// 与 window.innerHeight 对比
// window.innerHeight  -> 包含地址栏在内的总高度
// visualViewport.height -> 只有可见内容区域高度（推荐使用）
```

**主要功能**:
- ✅ 监听 `visualViewport` 变化（地址栏收缩时）
- ✅ 检测键盘弹起并自动滚动输入框
- ✅ 识别折叠屏设备（Samsung Fold、Google Pixel Fold）
- ✅ 处理屏幕方向变化
- ✅ Android特有的键盘处理
- ✅ 动态更新CSS变量供样式使用

**API方法**:
```javascript
// 获取实际可用高度
window.viewportAdapter.getUsableHeight();

// 获取实际可用宽度
window.viewportAdapter.getUsableWidth();

// 检查是否为折叠屏设备
window.viewportAdapter.isFoldable; // true/false

// 检查当前是否在折叠状态（仅Fold设备）
window.viewportAdapter.isFolded(); // true/false
```

---

### 2. `style-mobile.css` 改进

**关键改动**:

#### CSS变量系统
```css
:root {
    --viewport-height: 100vh;        /* 由JS动态设置 */
    --viewport-width: 100vw;
    --usable-height: 100vh;          /* 排除键盘高度 */
    
    /* Safe Area - 处理notch和折叠线 */
    --safe-area-inset-top: 0px;
    --safe-area-inset-bottom: 0px;
    --safe-area-inset-left: 0px;
    --safe-area-inset-right: 0px;
}
```

#### 身体（Body）样式
```css
html {
    /* dvh = Dynamic Viewport Height（动态，自动适应浏览器UI） */
    /* vh = Viewport Height（静态，不适应浏览器UI） */
    height: 100dvh; /* 推荐，自动调整 */
    height: 100vh;  /* 备选方案 */
}

body {
    /* 固定定位防止Android键盘推动页面 */
    position: fixed;
    top: 0;
    left: 0;
    right: 0;
    bottom: 0;
    height: var(--viewport-height);
    
    /* 处理notch和圆角屏 */
    padding-top: max(var(--gutter), env(safe-area-inset-top));
    padding-bottom: max(var(--gutter), env(safe-area-inset-bottom));
}
```

#### Safe Area Inset
```css
/* 用环境变量处理notch、刘海、折叠线等 */
padding: max(0, env(safe-area-inset-top));
```

---

### 3. `index.html` 改进

#### Viewport Meta标签
```html
<meta name="viewport" content="
    width=device-width,
    initial-scale=1.0,
    viewport-fit=cover,        <!-- 关键！让内容延伸到安全区边界 -->
    maximum-scale=5.0,
    user-scalable=yes
">

<!-- 支持webapps在全屏显示 -->
<meta name="apple-mobile-web-app-capable" content="yes">
<meta name="apple-mobile-web-app-status-bar-style" content="black-translucent">
```

#### Body样式
```html
<body style="
    height: 100dvh;  /* Dynamic Viewport Height - 推荐 -->
    height: 100vh;   <!-- 备选 -->
">
```

#### 容器大小
```html
<!-- 使用dvh确保完整填充 -->
<div id="mainApp" style="
    height: 100dvh;
    max-height: 100dvh;
">
```

---

## 🔧 工作流程

### 页面加载时
```
1. index.html加载viewport-adapter.js
2. JS初始化，检测设备类型
3. 设置CSS变量（--viewport-height等）
4. 监听resize和orientationchange事件
```

### 用户与输入框交互时
```
1. 输入框获得焦点 → focusin事件
2. JS自动滚动输入框到视口中心
3. 软键盘弹起 → visualViewport resize事件
4. JS更新CSS变量，容器自动重排（不需要手动处理）
```

### 折叠屏展开/折叠时
```
1. 屏幕方向改变 → orientationchange事件
2. JS检测宽度变化（外屏<600px，内屏>600px）
3. 更新布局自适应新屏幕尺寸
4. Safe Area变化 → env(safe-area-inset-*)自动适应
```

### 浏览器地址栏显示/隐藏时
```
1. 地址栏变化 → visualViewport resize事件（最关键！）
2. JS获取新的 visualViewport.height
3. 更新 --viewport-height CSS变量
4. 容器自动重新计算高度（CSS flex布局自动处理）
```

---

## 📊 兼容性表

| 浏览器 | dvh | svh | visualViewport | safe-area | 折叠屏检测 |
|---------|-----|-----|-----------------|-----------|----------|
| Chrome/Edge (Android) | ✅ 108+ | ✅ 108+ | ✅ | ✅ | ✅ |
| Firefox (Android) | ✅ | ✅ | ✅ | ✅ | ✅ |
| Safari (iOS) | ✅ 15.4+ | ✅ | ✅ | ✅ | ⚠️ 有限 |
| Samsung Browser | ✅ | ✅ | ✅ | ✅ | ✅ |

---

## 🎮 测试checklist

- [ ] **地址栏**: 打开页面→地址栏自动隐藏→页面自动填满屏幕
- [ ] **键盘**: 点击输入框→键盘弹起→输入框自动滚动到中心
- [ ] **方向**: 旋转设备→布局自动调整，无不必要的滚动
- [ ] **折叠屏**: Fold3外屏→内屏切换→布局平滑过渡
- [ ] **notch**: iPhone/Android刘海屏→内容避开刘海区域

---

## 💡 高级配置（可选）

### 1. 锁定竖屏（某些应用需要）
```javascript
// 启用竖屏锁定
window.viewportAdapter.lockPortrait();

// 解除锁定
window.viewportAdapter.unlockOrientation();
```

### 2. 禁用缩放
```html
<!-- 更严格的限制 -->
<meta name="viewport" content="width=device-width, initial-scale=1.0, user-scalable=no">
```

### 3. 自定义Safe Area距离
```css
:root {
    --safe-area-inset-top: 20px;      /* 自定义值 */
    --safe-area-inset-bottom: 30px;
}
```

### 4. 针对特定设备的样式
```css
/* 三星Fold检测 */
@supports (padding: max(0px, env(safe-area-inset-top))) {
    body {
        /* 特定样式 */
    }
}

/* 检查折叠线的CSS Media Query */
@media (fold-left: 0px) {
    /* 左侧有折叠线的样式 */
}

@media (fold-top: 0px) {
    /* 顶部有折叠线的样式 */
}
```

---

## 🐛 常见问题

### Q: 为什么要使用 visualViewport 而不是 window.innerHeight？
A: `window.innerHeight` 包含浏览器UI（地址栏），不准确。`visualViewport.height` 才是用户真实可见的高度。

### Q: 键盘弹起后页面变形怎么办？
A: 这是HTML中 `body { position: fixed; }` 的作用，防止Android键盘"推动"页面。

### Q: 折叠屏上外屏显示不完整？
A: 检查 `viewport-fit=cover` 和 `env(safe-area-inset-*)` 是否正确配置。

### Q: iOS和Android的表现不一样？
A: 正常的。iOS Safari和Android浏览器的UI机制不同。脚本已处理两者差异。

---

## 📚 参考资源

- [MDN: visualViewport API](https://developer.mozilla.org/en-US/docs/Web/API/VisualViewport)
- [MDN: CSS Environment Variables (env)](https://developer.mozilla.org/en-US/docs/Web/CSS/env)
- [W3C: Device Adaptation Module](https://www.w3.org/TR/css-device-adapt/)

---

## ✅ 总结

这个方案通过以下方式实现完整的手机适配：

1. **动态单位**: 使用 `dvh` 而非 `vh`
2. **实时监听**: 监听 `visualViewport` 而非 `resize`
3. **Smart滚动**: 输入框自动滚动到可见区域
4. **设备检测**: 识别折叠屏并采用相应策略
5. **Safe Area**: 自动避开notch和折叠线

**结果**: 用户无需滚动，所有UI始终可见和可交互！🎉
