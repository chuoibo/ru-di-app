/** Host state for Timeline | Journey. Lives above both views, never inside the map. */

import { useCallback, useState } from "react";

export type CheDoXem = "lich-trinh" | "hanh-trinh";

export function useCheDoLichTrinh() {
  const [cheDo, setCheDo] = useState<CheDoXem>("lich-trinh");
  const [selectedActivityId, setSelectedActivityId] = useState<string | null>(null);
  const [selectedSegmentId, setSelectedSegmentId] = useState<string | null>(null);
  const [fitDem, setFitDem] = useState(0);
  const [toiDem, setToiDem] = useState(0);

  const chonHoatDong = useCallback((id: string | null) => {
    setSelectedActivityId(id);
    setSelectedSegmentId(null);
    if (id) setToiDem((n) => n + 1);
  }, []);

  const chonDoan = useCallback((id: string | null) => {
    setSelectedSegmentId(id);
    setSelectedActivityId(null);
  }, []);

  const khopHanhTrinh = useCallback(() => {
    setFitDem((n) => n + 1);
  }, []);

  const userMove = useCallback(() => {
    /* Camera stays where the person left it; Fit Journey is the only yank. */
  }, []);

  const doiCheDo = useCallback((tiep: CheDoXem) => {
    setCheDo(tiep);
    if (tiep === "hanh-trinh") setFitDem((n) => n + 1);
  }, []);

  return {
    cheDo,
    doiCheDo,
    selectedActivityId,
    selectedSegmentId,
    fitDem,
    toiDem,
    chonHoatDong,
    chonDoan,
    khopHanhTrinh,
    userMove,
  };
}
