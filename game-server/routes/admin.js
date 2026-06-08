import { Router } from 'express'
import auth from '../middleware/auth.js'
import { saveJson, loadJson, listFiles } from '../controllers/fileController.js'

const router = Router()
router.use(auth)
router.post('/save-json', saveJson)
router.get('/load-json', loadJson)
router.get('/list-files', listFiles)

export default router