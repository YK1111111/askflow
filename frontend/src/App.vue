<script setup>
import { computed, onBeforeUnmount, onMounted, reactive, ref, watch } from 'vue'
import { request, streamNotifications, token } from './api'

const tabs = [
  { id: 'recommended', label: '推荐' },
  { id: 'following', label: '关注' },
  { id: 'hot', label: '热榜' }
]
const activeTab = ref('recommended')
const feed = ref([])
const nextCursor = ref('')
const loading = ref(false)
const message = ref('')
const loggedIn = ref(Boolean(token.get()))
const profile = ref(null)
const authOpen = ref(false)
const authMode = ref('login')
const auth = reactive({ username: '', password: '' })
const authBusy = ref(false)
const composeOpen = ref(false)
const questionForm = reactive({ title: '', content: '' })
const detail = ref(null)
const answers = ref([])
const answerText = ref('')
const notificationsOpen = ref(false)
const notifications = ref([])
const unread = ref(0)
let notificationController

const currentTitle = computed(() => tabs.find(item => item.id === activeTab.value)?.label || '')

function toast(text) {
  message.value = text
  window.setTimeout(() => { if (message.value === text) message.value = '' }, 2600)
}

async function loadFeed(append = false) {
  if (loading.value) return
  if (activeTab.value === 'following' && !loggedIn.value) {
    authOpen.value = true
    activeTab.value = 'recommended'
    return
  }
  loading.value = true
  try {
    const cursor = append && nextCursor.value ? `&cursor=${encodeURIComponent(nextCursor.value)}` : ''
    const data = await request(`/feeds/${activeTab.value}?limit=12${cursor}`)
    feed.value = append ? [...feed.value, ...(data.items || [])] : (data.items || [])
    nextCursor.value = data.next_cursor || ''
  } catch (error) {
    toast(error.message)
  } finally {
    loading.value = false
  }
}

async function submitAuth() {
  authBusy.value = true
  try {
    if (authMode.value === 'register') {
      await request('/users/register', { method: 'POST', body: JSON.stringify(auth) })
    }
    const data = await request('/users/login', { method: 'POST', body: JSON.stringify(auth) })
    token.set(data.access_token)
    loggedIn.value = true
    authOpen.value = false
    auth.password = ''
    await loadProfile()
    startNotifications()
    toast('欢迎回来')
  } catch (error) {
    toast(error.message)
  } finally {
    authBusy.value = false
  }
}

async function loadProfile() {
  if (!loggedIn.value) return
  try { profile.value = await request('/users/me') } catch { logout() }
}

function logout() {
  token.clear()
  loggedIn.value = false
  profile.value = null
  notifications.value = []
  unread.value = 0
  notificationController?.abort()
  toast('已退出登录')
}

async function publishQuestion() {
  try {
    const data = await request('/questions', { method: 'POST', body: JSON.stringify(questionForm) })
    composeOpen.value = false
    questionForm.title = ''
    questionForm.content = ''
    toast('问题发布成功')
    await openQuestion(data.id)
  } catch (error) { toast(error.message) }
}

async function openQuestion(id) {
  try {
    const [question, answerData] = await Promise.all([
      request(`/questions/${id}`), request(`/questions/${id}/answers?limit=30`)
    ])
    detail.value = question
    answers.value = answerData.items || []
    window.scrollTo({ top: 0, behavior: 'smooth' })
  } catch (error) { toast(error.message) }
}

async function publishAnswer() {
  if (!requireLogin()) return
  try {
    await request(`/questions/${detail.value.id}/answers`, { method: 'POST', body: JSON.stringify({ content: answerText.value }) })
    answerText.value = ''
    await openQuestion(detail.value.id)
    toast('回答发布成功')
  } catch (error) { toast(error.message) }
}

async function answerAction(answer, action) {
  if (!requireLogin()) return
  try {
    await request(`/answers/${answer.id}/${action}`, { method: 'POST' })
    if (action === 'like') answer.like_count++
    if (action === 'favorite') answer.favorite_count++
    toast(action === 'like' ? '已点赞' : '已收藏')
  } catch (error) { toast(error.message) }
}

async function comment(answer) {
  if (!requireLogin()) return
  const content = window.prompt('写下你的评论')
  if (!content) return
  try {
    await request(`/answers/${answer.id}/comments`, { method: 'POST', body: JSON.stringify({ content }) })
    answer.comment_count++
    toast('评论成功')
  } catch (error) { toast(error.message) }
}

function requireLogin() {
  if (loggedIn.value) return true
  authOpen.value = true
  toast('请先登录')
  return false
}

async function loadNotifications() {
  if (!loggedIn.value) return
  try {
    const data = await request('/notifications?limit=30')
    notifications.value = data.items || []
    unread.value = data.unread_count || 0
  } catch (error) { toast(error.message) }
}

async function markAllRead() {
  await request('/notifications/read-all', { method: 'POST' })
  notifications.value.forEach(item => { item.read_at ||= new Date().toISOString() })
  unread.value = 0
}

function startNotifications() {
  notificationController?.abort()
  if (!loggedIn.value) return
  notificationController = new AbortController()
  streamNotifications(event => {
    notifications.value.unshift(event)
    unread.value++
    toast(event.content)
  }, notificationController.signal).catch(error => {
    if (error.name !== 'AbortError' && loggedIn.value) window.setTimeout(startNotifications, 3000)
  })
}

function displayName(author) { return author?.nickname || author?.username || '匿名用户' }
function initials(author) { return displayName(author).slice(0, 1).toUpperCase() }
function date(value) { return value ? new Intl.DateTimeFormat('zh-CN', { month: 'short', day: 'numeric', hour: '2-digit', minute: '2-digit' }).format(new Date(value)) : '' }

watch(activeTab, () => loadFeed())
onMounted(async () => {
  await loadFeed()
  if (loggedIn.value) {
    await Promise.all([loadProfile(), loadNotifications()])
    startNotifications()
  }
})
onBeforeUnmount(() => notificationController?.abort())
</script>

<template>
  <header class="topbar">
    <button class="brand" @click="detail = null; activeTab = 'recommended'"><span>问</span> AskFlow</button>
    <nav>
      <button v-for="tab in tabs" :key="tab.id" :class="{ active: !detail && activeTab === tab.id }" @click="detail = null; activeTab = tab.id">{{ tab.label }}</button>
    </nav>
    <div class="top-actions">
      <button class="ask" @click="loggedIn ? composeOpen = true : authOpen = true">提出问题</button>
      <button v-if="loggedIn" class="icon-button" title="通知" @click="notificationsOpen = !notificationsOpen; loadNotifications()">🔔<b v-if="unread">{{ unread > 9 ? '9+' : unread }}</b></button>
      <button v-if="!loggedIn" class="login" @click="authOpen = true">登录</button>
      <div v-else class="user-menu"><span class="avatar small">{{ initials(profile) }}</span><span>{{ displayName(profile) }}</span><button @click="logout">退出</button></div>
    </div>
  </header>

  <main>
    <section v-if="!detail" class="layout">
      <div class="content">
        <div class="page-heading"><div><p class="eyebrow">DISCOVER IDEAS</p><h1>{{ currentTitle }} Feed</h1></div><button class="refresh" @click="loadFeed()">↻ 刷新</button></div>
        <div v-if="loading && !feed.length" class="state">正在整理内容…</div>
        <div v-else-if="!feed.length" class="state"><strong>这里还没有内容</strong><span>发布一个问题或回答，让社区流动起来。</span></div>
        <article v-for="(item, index) in feed" :key="item.question.id" class="feed-card" @click="openQuestion(item.question.id)">
          <div v-if="activeTab === 'hot'" class="rank">{{ String(index + 1).padStart(2, '0') }}</div>
          <div class="card-body">
            <div class="meta"><span class="avatar">{{ initials(item.answer?.author || item.question.author) }}</span><span>{{ displayName(item.answer?.author || item.question.author) }}</span><i>·</i><time>{{ date(item.answer?.created_at || item.created_at) }}</time><em v-if="item.following_author">已关注</em></div>
            <h2>{{ item.question.title }}</h2>
            <p>{{ item.answer?.summary || item.summary }}</p>
            <div class="stats" v-if="item.answer"><span>▲ {{ item.answer.like_count }}</span><span>◌ {{ item.answer.comment_count }} 评论</span><span>☆ {{ item.answer.favorite_count }} 收藏</span><small v-if="item.recommend_score">匹配度 {{ item.recommend_score.toFixed(1) }}</small></div>
            <div class="stats" v-else><span>🔥 {{ item.hot_score.toFixed(1) }}</span><span>{{ item.stats.answer_count_24h }} 个新回答</span><span>{{ item.stats.view_count_24h }} 次浏览</span></div>
          </div>
        </article>
        <button v-if="nextCursor" class="load-more" :disabled="loading" @click="loadFeed(true)">{{ loading ? '加载中…' : '加载更多' }}</button>
      </div>
      <aside><div class="aside-card"><p class="eyebrow">ASKFLOW</p><h3>好问题，会找到好答案。</h3><p>关注你感兴趣的人，在讨论中沉淀知识。</p><button @click="loggedIn ? composeOpen = true : authOpen = true">开始提问 →</button></div><div class="aside-note">今日阅读<br><strong>{{ feed.length }}</strong><span> 条精选内容</span></div></aside>
    </section>

    <section v-else class="question-page">
      <button class="back" @click="detail = null">← 返回 {{ currentTitle }}</button>
      <div class="question-card"><p class="eyebrow">QUESTION</p><h1>{{ detail.title }}</h1><p class="question-content">{{ detail.content }}</p><div class="meta"><span class="avatar">{{ initials(detail.author) }}</span><span>{{ displayName(detail.author) }}</span><i>·</i><time>{{ date(detail.created_at) }}</time></div></div>
      <div class="answer-compose"><h3>写下你的回答</h3><textarea v-model="answerText" placeholder="分享你的经验、判断与思考…"></textarea><button @click="publishAnswer">发布回答</button></div>
      <h3 class="answer-count">{{ answers.length }} 个回答</h3>
      <article v-for="answer in answers" :key="answer.id" class="answer-card"><div class="meta"><span class="avatar">{{ initials(answer.author) }}</span><span>{{ displayName(answer.author) }}</span><em v-if="answer.pinned">置顶回答</em></div><p>{{ answer.content }}</p><div class="stats"><button @click="answerAction(answer, 'like')">▲ {{ answer.like_count }} 赞同</button><button @click="comment(answer)">◌ {{ answer.comment_count }} 评论</button><button @click="answerAction(answer, 'favorite')">☆ {{ answer.favorite_count }} 收藏</button><time>{{ date(answer.created_at) }}</time></div></article>
    </section>
  </main>

  <div v-if="authOpen || composeOpen" class="overlay" @click.self="authOpen = composeOpen = false">
    <form v-if="authOpen" class="modal auth-modal" @submit.prevent="submitAuth"><button type="button" class="close" @click="authOpen = false">×</button><p class="eyebrow">WELCOME TO ASKFLOW</p><h2>{{ authMode === 'login' ? '登录你的账号' : '加入 AskFlow' }}</h2><label>用户名<input v-model="auth.username" required autocomplete="username" placeholder="例如 linus" /></label><label>密码<input v-model="auth.password" required minlength="8" type="password" autocomplete="current-password" placeholder="至少 8 位" /></label><button class="primary" :disabled="authBusy">{{ authBusy ? '请稍候…' : authMode === 'login' ? '登录' : '注册并登录' }}</button><button type="button" class="switch" @click="authMode = authMode === 'login' ? 'register' : 'login'">{{ authMode === 'login' ? '没有账号？立即注册' : '已有账号？返回登录' }}</button></form>
    <form v-else class="modal compose-modal" @submit.prevent="publishQuestion"><button type="button" class="close" @click="composeOpen = false">×</button><p class="eyebrow">NEW QUESTION</p><h2>提出一个好问题</h2><label>标题<input v-model="questionForm.title" required maxlength="100" placeholder="一句话说清楚你的问题" /></label><label>补充说明<textarea v-model="questionForm.content" required placeholder="提供背景、已经尝试过的方法…"></textarea></label><button class="primary">发布问题</button></form>
  </div>

  <aside v-if="notificationsOpen" class="notification-panel"><div class="panel-head"><div><p class="eyebrow">INBOX</p><h3>通知</h3></div><button @click="notificationsOpen = false">×</button></div><button v-if="unread" class="read-all" @click="markAllRead">全部标记已读</button><div v-if="!notifications.length" class="state small-state">暂无通知</div><div v-for="item in notifications" :key="item.id" class="notice" :class="{ unread: !item.read_at }"><span></span><div><strong>{{ item.content }}</strong><time>{{ date(item.created_at) }}</time></div></div></aside>
  <transition name="toast"><div v-if="message" class="toast">{{ message }}</div></transition>
</template>
