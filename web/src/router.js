import { createRouter, createWebHistory } from 'vue-router'
import Home from './views/Home.vue'
import Clip from './views/Clip.vue'
import Settings from './views/Settings.vue'

export default createRouter({
  history: createWebHistory(),
  routes: [
    { path: '/', component: Home },
    { path: '/clip', component: Clip },
    { path: '/settings', component: Settings }
  ]
})
