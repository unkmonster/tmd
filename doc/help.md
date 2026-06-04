# 获取 Cookie
1. 使用 Chrome 浏览器打开 https://twitter.com 后，按`F12` 打开开发者控制台
2. 选中顶部 `应用`，并复制对应项的值

​	![ 2024-06-25 093928.png](https://s2.loli.net/2024/06/25/O6PwWGoqYLZAJXc.png)

# 获取 list_id, user_id, screen_name
## list_id

![image-20240622184027270.png](https://s2.loli.net/2024/06/22/M4xmVUkZ6DpPrds.png "list_id")

## 用户的 screen_name

![ 2024-06-22 185026.png](https://s2.loli.net/2024/06/22/u45c1nUwHOKtbjE.png "用户的screen_name")

## user_id

user_id 是 Twitter 用户的唯一数字标识符。可以通过以下方式获取：

1. 使用第三方工具如 [twitter-id](https://twitter-id.vercel.app/) 查找
2. 通过 API 响应中的 `user.id` 字段获取
3. 在浏览器开发者工具的 Network 标签页中，查看 Twitter API 请求响应中的 `rest_id` 字段
