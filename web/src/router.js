import { createRouter, createWebHistory } from 'vue-router'
import Home from './views/Home.vue'
import Clip from './views/Clip.vue'
import Settings from './views/Settings.vue'
import ToolShell from './components/ToolShell.vue'

export default createRouter({
  history: createWebHistory(),
  routes: [
    { path: '/', component: Home },
    {
      path: '/clip',
      component: Clip,
      children: [{ path: ':id', component: ToolShell }]
    },
    { path: '/settings', component: Settings }
  ]
})
