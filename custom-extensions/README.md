# 自定义扩展目录

把你自己的 JavaScript 扩展脚本放到这里即可。

- 文件名以 `.js` 结尾
- 启动后页面会自动请求 `/api/custom/extensions` 并发现目录下的脚本文件
- 程序会把每个脚本附加到页面正文中，以 `script.src = /custom-extensions/<filename>` 的方式加载

示例：

```js
// 在这个目录放你自己的扩展脚本
// 例如：hello.js
```

不要直接写入页面内联脚本。
