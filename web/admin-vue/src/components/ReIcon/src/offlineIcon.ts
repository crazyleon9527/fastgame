// 本地菜单图标，在 layout/index.vue 首屏加载，避免 Iconify 离线警告
import { getSvgInfo } from "@pureadmin/utils";
import { addIcon } from "@iconify/vue/dist/offline";

import EpHomeFilled from "~icons/ep/home-filled?raw";
import EpWarningFilled from "~icons/ep/warning-filled?raw";
import EpOfficeBuilding from "~icons/ep/office-building?raw";
import EpVideoPlay from "~icons/ep/video-play?raw";
import EpDataAnalysis from "~icons/ep/data-analysis?raw";
import EpSearch from "~icons/ep/search?raw";
import EpSetting from "~icons/ep/setting?raw";

import RiSearchLine from "~icons/ri/search-line?raw";
import RiInformationLine from "~icons/ri/information-line?raw";

const icons = [
  ["ep/home-filled", EpHomeFilled],
  ["ep/warning-filled", EpWarningFilled],
  ["ep/office-building", EpOfficeBuilding],
  ["ep/video-play", EpVideoPlay],
  ["ep/data-analysis", EpDataAnalysis],
  ["ep/search", EpSearch],
  ["ep/setting", EpSetting],
  ["ri/search-line", RiSearchLine],
  ["ri/information-line", RiInformationLine]
];

icons.forEach(([name, icon]) => {
  addIcon(name as string, getSvgInfo(icon as string));
});
