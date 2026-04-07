import { createRouter, createWebHashHistory } from 'vue-router'
import GridView from './views/GridView.vue'
import AgentView from './views/AgentView.vue'

export default createRouter({
  history: createWebHashHistory(),
  routes: [
    { path: '/', component: GridView },
    { path: '/agent/:id', component: AgentView },
    { path: '/:pathMatch(.*)*', redirect: '/' },
  ],
})
