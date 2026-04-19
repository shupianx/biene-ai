import { createRouter, createWebHashHistory } from 'vue-router'
import GridView from './views/GridView.vue'
import AgentView from './views/AgentView.vue'
import SkillsView from './views/SkillsView.vue'

export default createRouter({
  history: createWebHashHistory(),
  routes: [
    { path: '/', component: GridView },
    { path: '/agent/:id', component: AgentView },
    { path: '/skills', component: SkillsView },
    { path: '/:pathMatch(.*)*', redirect: '/' },
  ],
})
