import { createRouter, createWebHashHistory } from 'vue-router'
import { isAuthenticated } from './services/auth'

const routes = [
  {
    path: '/login',
    name: 'Login',
    component: () => import('./views/LoginView.vue')
  },
  {
    path: '/',
    name: 'Home',
    component: () => import('./views/HomeView.vue'),
    meta: { requiresAuth: true }
  },
  {
    path: '/chat/:conversationId',
    name: 'Chat',
    component: () => import('./views/ChatView.vue'),
    meta: { requiresAuth: true }
  },
  {
    path: '/new-chat',
    name: 'NewChat',
    component: () => import('./views/NewChatView.vue'),
    meta: { requiresAuth: true }
  },
  {
    path: '/new-group',
    name: 'NewGroup',
    component: () => import('./views/NewGroupView.vue'),
    meta: { requiresAuth: true }
  },
  {
    path: '/profile',
    name: 'Profile',
    component: () => import('./views/ProfileView.vue'),
    meta: { requiresAuth: true }
  }
]

const router = createRouter({
  history: createWebHashHistory(),
  routes
})

router.beforeEach((to, from, next) => {
  if (to.meta.requiresAuth && !isAuthenticated()) {
    next('/login')
  } else if (to.path === '/login' && isAuthenticated()) {
    next('/')
  } else {
    next()
  }
})

export default router
