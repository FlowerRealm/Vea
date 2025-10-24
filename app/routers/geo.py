from __future__ import annotations

from fastapi import APIRouter, Depends, HTTPException, status
from sqlmodel import Session

from ..database import get_session
from ..schemas import GeoResourceCreate, GeoResourceRead, GeoResourceUpdate
from ..services import geo_service

router = APIRouter(prefix="/geo", tags=["geo"])


@router.get("/", response_model=list[GeoResourceRead])
def list_resources(session: Session = Depends(get_session)) -> list[GeoResourceRead]:
    return list(geo_service.list_resources(session))


@router.post("/", response_model=GeoResourceRead, status_code=status.HTTP_201_CREATED)
def create_resource(
    payload: GeoResourceCreate,
    session: Session = Depends(get_session),
) -> GeoResourceRead:
    return geo_service.create_resource(session, payload)


@router.put("/{resource_id}", response_model=GeoResourceRead)
def update_resource(
    resource_id: int,
    payload: GeoResourceUpdate,
    session: Session = Depends(get_session),
) -> GeoResourceRead:
    resource = geo_service.get_resource(session, resource_id)
    if not resource:
        raise HTTPException(status_code=status.HTTP_404_NOT_FOUND, detail="Resource not found")
    return geo_service.update_resource(session, resource, payload)


@router.delete("/{resource_id}", status_code=status.HTTP_204_NO_CONTENT)
def delete_resource(resource_id: int, session: Session = Depends(get_session)) -> None:
    resource = geo_service.get_resource(session, resource_id)
    if not resource:
        raise HTTPException(status_code=status.HTTP_404_NOT_FOUND, detail="Resource not found")
    geo_service.delete_resource(session, resource)


@router.post("/{resource_id}/refresh", response_model=GeoResourceRead)
def refresh_resource(resource_id: int, session: Session = Depends(get_session)) -> GeoResourceRead:
    resource = geo_service.get_resource(session, resource_id)
    if not resource:
        raise HTTPException(status_code=status.HTTP_404_NOT_FOUND, detail="Resource not found")
    return geo_service.refresh_resource(session, resource)
